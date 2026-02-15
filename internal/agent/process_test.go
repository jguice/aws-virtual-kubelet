package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testPod(command []string, args []string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   "test-image",
					Command: command,
					Args:    args,
				},
			},
		},
	}
}

func TestNewProcessManager(t *testing.T) {
	pm := NewProcessManager("/tmp/test-logs")
	if pm.State() != StateIdle {
		t.Errorf("expected StateIdle, got %d", pm.State())
	}
	if pm.IsRunning() {
		t.Error("expected not running")
	}
}

func TestLaunchAndTerminate(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod([]string{"sleep"}, []string{"60"})

	// Launch
	err := pm.Launch(pod)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	if !pm.IsRunning() {
		t.Error("expected process to be running")
	}

	if pm.Pod() == nil {
		t.Error("expected pod to be set")
	}

	if pm.StartTime().IsZero() {
		t.Error("expected start time to be set")
	}

	// Verify log files were created
	stdoutPath, stderrPath := pm.LogPaths()
	if stdoutPath == "" || stderrPath == "" {
		t.Error("expected log paths to be set")
	}

	// Terminate
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = pm.Terminate(ctx)
	if err != nil {
		t.Fatalf("Terminate failed: %v", err)
	}

	// Wait briefly for state to update
	time.Sleep(100 * time.Millisecond)

	if pm.IsRunning() {
		t.Error("expected process to not be running after terminate")
	}
}

func TestLaunchWithEnvVars(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod([]string{"/bin/sh"}, []string{"-c", "echo $TEST_VAR"})
	pod.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "TEST_VAR", Value: "hello-world"},
	}

	err := pm.Launch(pod)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	// Wait for process to finish (it's a quick echo)
	time.Sleep(500 * time.Millisecond)

	if pm.State() != StateTerminated {
		t.Errorf("expected StateTerminated, got %d", pm.State())
	}

	if pm.ExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", pm.ExitCode())
	}

	// Check stdout log contains the env var value
	stdoutPath, _ := pm.LogPaths()
	content, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatalf("failed to read stdout log: %v", err)
	}
	if string(content) != "hello-world\n" {
		t.Errorf("expected 'hello-world\\n' in stdout, got %q", string(content))
	}
}

func TestLaunchNoContainers(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{},
	}

	err := pm.Launch(pod)
	if err == nil {
		t.Error("expected error for pod with no containers")
	}
}

func TestLaunchNoCommand(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod(nil, nil)

	err := pm.Launch(pod)
	if err == nil {
		t.Error("expected error for container with no command")
	}
}

func TestLaunchDuplicateReject(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod([]string{"sleep"}, []string{"60"})

	err := pm.Launch(pod)
	if err != nil {
		t.Fatalf("first Launch failed: %v", err)
	}
	defer pm.Terminate(context.Background())

	// Second launch should fail
	err = pm.Launch(pod)
	if err == nil {
		t.Error("expected error on duplicate launch")
	}
}

func TestProcessExitCode(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod([]string{"/bin/sh"}, []string{"-c", "exit 42"})

	err := pm.Launch(pod)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	// Wait for process to exit
	time.Sleep(500 * time.Millisecond)

	if pm.State() != StateTerminated {
		t.Errorf("expected StateTerminated, got %d", pm.State())
	}

	if pm.ExitCode() != 42 {
		t.Errorf("expected exit code 42, got %d", pm.ExitCode())
	}
}

func TestLogFileCreation(t *testing.T) {
	logDir := t.TempDir()
	pm := NewProcessManager(logDir)

	pod := testPod([]string{"/bin/sh"}, []string{"-c", "echo stdout-test; echo stderr-test >&2"})

	err := pm.Launch(pod)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Check log files exist
	expectedStdout := filepath.Join(logDir, "default_test-pod_test-container.stdout.log")
	expectedStderr := filepath.Join(logDir, "default_test-pod_test-container.stderr.log")

	if _, err := os.Stat(expectedStdout); os.IsNotExist(err) {
		t.Errorf("stdout log file not created at %s", expectedStdout)
	}
	if _, err := os.Stat(expectedStderr); os.IsNotExist(err) {
		t.Errorf("stderr log file not created at %s", expectedStderr)
	}

	stdout, _ := os.ReadFile(expectedStdout)
	stderr, _ := os.ReadFile(expectedStderr)

	if string(stdout) != "stdout-test\n" {
		t.Errorf("expected 'stdout-test\\n', got %q", string(stdout))
	}
	if string(stderr) != "stderr-test\n" {
		t.Errorf("expected 'stderr-test\\n', got %q", string(stderr))
	}
}

func TestTerminateNoProcess(t *testing.T) {
	pm := NewProcessManager(t.TempDir())

	// Should not error when nothing is running
	err := pm.Terminate(context.Background())
	if err != nil {
		t.Errorf("Terminate with no process should not error, got: %v", err)
	}
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      []string
		args     []string
		expected []string
	}{
		{"command only", []string{"/bin/bash"}, nil, []string{"/bin/bash"}},
		{"args only", nil, []string{"arg1", "arg2"}, []string{"arg1", "arg2"}},
		{"both", []string{"/bin/sh", "-c"}, []string{"echo hi"}, []string{"/bin/sh", "-c", "echo hi"}},
		{"empty", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{Command: tt.cmd, Args: tt.args}
			result := buildCommand(c)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	c := corev1.Container{
		Env: []corev1.EnvVar{
			{Name: "FOO", Value: "bar"},
			{Name: "BAZ", Value: "qux"},
		},
	}

	env := buildEnv(c)

	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitEnvVar(e)
		if parts != nil {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", envMap["FOO"])
	}
	if envMap["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%s", envMap["BAZ"])
	}
}

func splitEnvVar(e string) []string {
	for i, c := range e {
		if c == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return nil
}
