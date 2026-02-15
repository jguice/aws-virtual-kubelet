/*
Process manager for the VKVMA agent.

Manages the lifecycle of a native workload process on the EC2 instance.
Maps Kubernetes pod container specs to OS-level process execution:
  - container.Command  → process binary
  - container.Args     → process arguments
  - container.Env      → process environment variables
  - container.WorkingDir → process working directory
*/
package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// ProcessState represents the current state of the managed workload process.
type ProcessState int

const (
	// StateIdle means no process has been launched yet.
	StateIdle ProcessState = iota
	// StateRunning means the process is actively running.
	StateRunning
	// StateTerminated means the process exited (successfully or not).
	StateTerminated
	// StateFailed means the process could not be started or crashed.
	StateFailed

	// gracefulTermTimeout is how long we wait after SIGTERM before sending SIGKILL.
	gracefulTermTimeout = 30 * time.Second
)

// ProcessManager manages the lifecycle of a single workload process.
type ProcessManager struct {
	mu sync.RWMutex

	state     ProcessState
	cmd       *exec.Cmd
	pod       *corev1.Pod
	logDir    string
	startTime time.Time
	exitCode  int
	exitErr   error

	// Log file handles and paths
	stdout     *os.File
	stderr     *os.File
	stdoutPath string
	stderrPath string

	// done is closed when waitForExit completes
	done chan struct{}
}

// NewProcessManager creates a new ProcessManager that writes workload logs to logDir.
func NewProcessManager(logDir string) *ProcessManager {
	return &ProcessManager{
		state:  StateIdle,
		logDir: logDir,
	}
}

// Launch starts a workload process based on the pod's first container spec.
// Returns an error if a process is already running or the spec is invalid.
func (pm *ProcessManager) Launch(pod *corev1.Pod) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.state == StateRunning {
		return fmt.Errorf("process already running (pid %d)", pm.cmd.Process.Pid)
	}

	if len(pod.Spec.Containers) == 0 {
		return fmt.Errorf("pod %s/%s has no containers", pod.Namespace, pod.Name)
	}

	container := pod.Spec.Containers[0]

	// Determine the command to run
	cmdArgs := buildCommand(container)
	if len(cmdArgs) == 0 {
		return fmt.Errorf("container %q has no command or args specified", container.Name)
	}

	log.Printf("launching workload: %v", cmdArgs)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	// Set environment variables
	cmd.Env = buildEnv(container)

	// Set working directory
	if container.WorkingDir != "" {
		cmd.Dir = container.WorkingDir
	}

	// Set up log capture
	stdout, stderr, err := pm.openLogFiles(pod, container.Name)
	if err != nil {
		return fmt.Errorf("failed to open log files: %w", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Start the process in its own process group so we can signal the whole group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		pm.closeLogFiles()
		pm.state = StateFailed
		pm.exitErr = err
		return fmt.Errorf("failed to start process: %w", err)
	}

	pm.cmd = cmd
	pm.pod = pod
	pm.state = StateRunning
	pm.startTime = time.Now()
	pm.exitCode = 0
	pm.exitErr = nil
	pm.done = make(chan struct{})

	log.Printf("workload started (pid %d)", cmd.Process.Pid)

	// Monitor the process in the background
	go pm.waitForExit()

	return nil
}

// Terminate gracefully stops the running workload.
// Sends SIGTERM, waits up to gracefulTermTimeout, then sends SIGKILL.
func (pm *ProcessManager) Terminate(ctx context.Context) error {
	pm.mu.RLock()
	if pm.state != StateRunning || pm.cmd == nil || pm.cmd.Process == nil {
		pm.mu.RUnlock()
		log.Printf("no running process to terminate")
		return nil
	}
	pid := pm.cmd.Process.Pid
	pm.mu.RUnlock()

	log.Printf("terminating workload (pid %d) with SIGTERM", pid)

	// Send SIGTERM to the process group
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		log.Printf("SIGTERM failed: %v, attempting SIGKILL", err)
		return pm.forceKill(pid)
	}

	// Wait for waitForExit goroutine to detect process exit
	pm.mu.RLock()
	done := pm.done
	pm.mu.RUnlock()

	select {
	case <-done:
		log.Printf("workload terminated gracefully")
		return nil
	case <-time.After(gracefulTermTimeout):
		log.Printf("graceful shutdown timed out after %v, sending SIGKILL", gracefulTermTimeout)
		return pm.forceKill(pid)
	case <-ctx.Done():
		log.Printf("context cancelled, sending SIGKILL")
		return pm.forceKill(pid)
	}
}

// State returns the current process state.
func (pm *ProcessManager) State() ProcessState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.state
}

// Pod returns the current pod being managed.
func (pm *ProcessManager) Pod() *corev1.Pod {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.pod
}

// StartTime returns when the process was started.
func (pm *ProcessManager) StartTime() time.Time {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.startTime
}

// ExitCode returns the process exit code (only meaningful after termination).
func (pm *ProcessManager) ExitCode() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.exitCode
}

// ExitError returns the error from process exit (nil if clean exit).
func (pm *ProcessManager) ExitError() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.exitErr
}

// IsRunning returns true if the workload process is currently running.
func (pm *ProcessManager) IsRunning() bool {
	return pm.State() == StateRunning
}

// LogPaths returns the paths to the stdout and stderr log files.
func (pm *ProcessManager) LogPaths() (stdoutPath, stderrPath string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.stdoutPath, pm.stderrPath
}

// waitForExit monitors the process and updates state when it exits.
func (pm *ProcessManager) waitForExit() {
	pm.mu.RLock()
	cmd := pm.cmd
	done := pm.done
	pm.mu.RUnlock()

	err := cmd.Wait()

	pm.mu.Lock()
	pm.closeLogFiles()

	if err != nil {
		pm.exitErr = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			pm.exitCode = exitErr.ExitCode()
			pm.state = StateTerminated
			log.Printf("workload exited with code %d", pm.exitCode)
		} else {
			pm.state = StateFailed
			log.Printf("workload failed: %v", err)
		}
	} else {
		pm.exitCode = 0
		pm.state = StateTerminated
		log.Printf("workload exited successfully")
	}
	pm.mu.Unlock()

	close(done)
}

// forceKill sends SIGKILL to the process group.
func (pm *ProcessManager) forceKill(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("SIGKILL failed for pid %d: %w", pid, err)
	}
	log.Printf("SIGKILL sent to process group %d", pid)
	return nil
}

// openLogFiles creates log files for the workload's stdout and stderr.
func (pm *ProcessManager) openLogFiles(pod *corev1.Pod, containerName string) (*os.File, *os.File, error) {
	prefix := fmt.Sprintf("%s_%s_%s", pod.Namespace, pod.Name, containerName)

	stdoutPath := filepath.Join(pm.logDir, prefix+".stdout.log")
	stderrPath := filepath.Join(pm.logDir, prefix+".stderr.log")

	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open stdout log: %w", err)
	}

	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		stdout.Close()
		return nil, nil, fmt.Errorf("open stderr log: %w", err)
	}

	pm.stdout = stdout
	pm.stderr = stderr
	pm.stdoutPath = stdoutPath
	pm.stderrPath = stderrPath

	log.Printf("workload logs: stdout=%s stderr=%s", stdoutPath, stderrPath)
	return stdout, stderr, nil
}

// closeLogFiles closes open log file handles.
func (pm *ProcessManager) closeLogFiles() {
	if pm.stdout != nil {
		pm.stdout.Close()
		pm.stdout = nil
	}
	if pm.stderr != nil {
		pm.stderr.Close()
		pm.stderr = nil
	}
}

// buildCommand extracts the command and args from a container spec.
// Kubernetes convention: Command overrides ENTRYPOINT, Args overrides CMD.
func buildCommand(container corev1.Container) []string {
	var cmdArgs []string

	if len(container.Command) > 0 {
		cmdArgs = append(cmdArgs, container.Command...)
	}
	if len(container.Args) > 0 {
		cmdArgs = append(cmdArgs, container.Args...)
	}

	return cmdArgs
}

// buildEnv converts container env vars to OS environment format ("KEY=VALUE").
// Inherits the agent's environment and overlays the container's env vars.
func buildEnv(container corev1.Container) []string {
	// Start with the current process environment
	env := os.Environ()

	// Build a map for dedup (container env overrides inherited env)
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Overlay container env vars
	for _, e := range container.Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
		// Note: ValueFrom (fieldRef, configMapKeyRef, etc.) requires kubelet-level
		// resolution. For now we only support literal values. Future: resolve these
		// from the pod spec or inject from the provider side.
	}

	// Convert back to slice
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}
