# Demo App — VKVMA Agent Example

A minimal HTTP server that demonstrates the production VKVMA agent running a native
process on an EC2 instance, managed as a Kubernetes pod.

## How It Works

```
┌─────────────────────┐         gRPC          ┌─────────────────────────┐
│  Virtual Kubelet     │ ──────────────────── │  EC2 Instance            │
│  (Kubernetes node)   │  LaunchApplication   │                          │
│                      │ ──────────────────►  │  vkvmagent               │
│  Pod: demo-app       │                      │    └── demo-app (pid N)  │
│    container.command  │ ◄────────────────── │        ├── stdout.log    │
│    = /opt/vkvma/     │  PodStatus: Running  │        └── stderr.log    │
│      demo-app        │                      │                          │
└─────────────────────┘                       └─────────────────────────┘
```

1. You `kubectl apply` a pod spec with `container.command` pointing to a binary path
2. The VK provider launches an EC2 instance and calls `LaunchApplication` via gRPC
3. The agent on the EC2 instance executes the binary as a native OS process
4. The agent reports process health back as Kubernetes `PodStatus`
5. `kubectl delete pod` triggers `TerminateApplication` → SIGTERM → process exits

## Build

```bash
# Build the demo app (for the target EC2 architecture)
GOOS=linux GOARCH=arm64 go build -o demo-app ./examples/demo-app/

# Build the production agent
GOOS=linux GOARCH=arm64 go build -o vkvmagent ./cmd/vkvmagent/
```

For x86 instances, use `GOARCH=amd64`. For Mac Metal, use `GOOS=darwin GOARCH=arm64`.

## Deploy to EC2

Copy both binaries to the EC2 instance:

```bash
scp demo-app vkvmagent ec2-user@<instance-ip>:/opt/vkvma/
```

Start the agent:

```bash
ssh ec2-user@<instance-ip> '/opt/vkvma/vkvmagent --port 8200 --log-dir /var/log/vkvma'
```

## Apply the Pod

```bash
# Standard Linux instance
kubectl apply -f examples/pods/demo-app-pod.yaml

# Mac Metal (Apple Silicon)
kubectl apply -f examples/pods/demo-app-mac-metal-pod.yaml
```

## Verify

```bash
# Check pod status
kubectl get pod demo-app

# Check the app is serving
curl http://<instance-ip>:8080/
# → Hello from hello-vk!
#   Hostname: ip-10-0-1-42
#   Uptime: 2m15s
#   PID: 12345

# Health check
curl http://<instance-ip>:8080/healthz
# → ok

# View agent logs on the instance
ssh ec2-user@<instance-ip> 'cat /var/log/vkvma/default_demo-app_demo-http-server.stdout.log'
```

## Pod Spec Mapping

| Pod Spec Field        | Agent Behavior                          |
|-----------------------|-----------------------------------------|
| `container.command`   | Binary path to execute                  |
| `container.args`      | Arguments passed to the binary          |
| `container.env`       | Environment variables for the process   |
| `container.workingDir`| Working directory for the process       |
| `container.name`      | Used in log file naming                 |
| `image`               | Informational only (not pulled)         |

## Environment Variables

| Variable   | Default  | Description            |
|------------|----------|------------------------|
| `APP_PORT` | `8080`   | HTTP listen port       |
| `APP_NAME` | `vk-demo`| Name shown in responses|
