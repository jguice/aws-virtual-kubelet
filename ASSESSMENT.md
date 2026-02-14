# Project Assessment: aws-virtual-kubelet

**Date:** February 2026
**Last Release:** v0.5.3 (May 2022)
**Last Commit:** September 2022

---

## TL;DR

This project is **largely obsolete for its original intent** but retains niche value for a
narrow set of use cases. AWS's native EKS+Fargate integration superseded the serverless-pods
use case, and the upstream Virtual Kubelet ecosystem is on life support. However, the core
concept -- managing non-container EC2 workloads (especially macOS) through Kubernetes
primitives -- has no direct replacement and could still serve teams with that specific need,
provided significant modernization effort.

---

## What This Project Does

AWS Virtual Kubelet is a Kubernetes Virtual Kubelet provider that maps pod lifecycle
operations to EC2 instance management. When you create a pod, it launches an EC2 instance
with a gRPC agent (VKVMA); when you delete the pod, it terminates the instance. It includes:

- Pod-to-EC2 lifecycle mapping via the `PodLifecycleHandler` interface
- ENI-based virtual node networking for Kubernetes discovery
- gRPC agent protocol for workload management on instances
- Warm pool for pre-provisioning expensive instances (e.g., `mac1.metal`)
- Health monitoring with configurable checks
- Prometheus metrics (50+ metrics)
- AWS CDK infrastructure-as-code deployment

---

## Current State of the Ecosystem

| Component | Status |
|---|---|
| This project (awslabs/aws-virtual-kubelet) | Inactive since Sept 2022, not archived |
| virtual-kubelet/virtual-kubelet (upstream) | CNCF Sandbox, sporadic maintenance, v1.12.0 (Jan 2025) |
| virtual-kubelet/node-cli | **Archived** (formally retired) |
| virtual-kubelet/aws-fargate | Inactive, seeking maintainers |
| AWS EKS + Fargate | Active, production-ready, native integration |
| interLink (CNCF Sandbox) | Emerging successor framework for VK providers |

### Key ecosystem shifts since 2022:
- AWS built **native Fargate profiles** into EKS, eliminating the need for a VK-based Fargate provider
- **Karpenter** became the preferred EKS autoscaler for dynamic EC2 provisioning
- **interLink** (CNCF Sandbox, March 2025) emerged as a next-gen framework for building VK-like providers
- Oracle is the only major cloud offering a fully managed Virtual Kubelet product (OCI Virtual Nodes)
- Azure ACI provider remains the most actively maintained VK provider

---

## Technical Debt

| Issue | Severity |
|---|---|
| Go 1.14 declared in go.mod (EOL 2023) | High |
| Kubernetes client libraries pinned to v0.19.10 (K8s 1.19, EOL Aug 2021) | High |
| node-cli dependency v0.7.0 is now archived | High |
| AWS SDK v2 at v1.9.0 (mid-2021, current is v1.27+) | Medium |
| gRPC v1.44.0 (late 2021) | Low |
| Prometheus client v1.11.0 (early 2021) | Low |
| CI uses setup-go@v2 (deprecated, current is v5) | Low |
| Dockerfile uses golang:1.16-alpine | Low |
| Explicitly marked "non-production-ready" | -- |

Bringing this project to a modern baseline would require updating Go to 1.22+, Kubernetes
libraries to v0.29+, replacing the archived node-cli, and updating the AWS SDK. This is a
non-trivial effort due to breaking API changes across those major version bumps.

---

## Use Cases: Where This Project Could Still Be Useful

### 1. macOS CI/CD on Kubernetes (Viable, Niche)

**The original primary use case.** AWS offers `mac1.metal` and `mac2.metal` EC2 dedicated
hosts for macOS workloads. These cannot run in containers. This project enables managing
macOS build agents (Xcode builds, iOS testing) through standard Kubernetes tooling.

**Why it still matters:** There is no native Kubernetes solution for macOS workloads.
Fargate doesn't support macOS. Karpenter doesn't manage dedicated hosts well. Teams running
iOS/macOS CI alongside Linux container workloads in EKS have no better Kubernetes-native
option.

**Caveats:** Dedicated hosts have a 24-hour minimum allocation, making the warm pool feature
important but expensive. Alternative approaches (Anka, Orka, GitHub Actions macOS runners)
may be simpler.

### 2. Non-Containerizable Workloads on EC2 (Viable, Niche)

Some workloads resist containerization: legacy applications with kernel dependencies,
GPU-intensive workloads needing bare-metal access, applications requiring specific OS
configurations, or licensed software tied to machine identity.

This project lets teams manage these workloads through Kubernetes without containerizing
them, keeping a single orchestration plane.

**Caveats:** Most organizations handle this with separate EC2 management tooling (Terraform,
ASGs, Systems Manager) rather than forcing it through Kubernetes.

### 3. Reference Architecture for Custom VK Providers (Viable)

The codebase is well-documented with RFCs, comprehensive health monitoring, warm pool
design, and a clean gRPC agent protocol. It serves as a solid reference for teams building
their own Virtual Kubelet providers.

**Caveats:** The interLink project (CNCF Sandbox) now provides a more modern and accessible
framework for this purpose.

### 4. HPC/Burst Compute via Kubernetes (Marginal)

Using Kubernetes to burst workloads onto specialized EC2 instances (HPC, GPU clusters)
while maintaining a unified control plane.

**Caveats:** Karpenter handles this better for standard EC2 instance types. The interLink
project specifically targets HPC/Slurm bridging.

### 5. Edge/Hybrid Deployments (Marginal)

Managing EC2 instances in specialized VPC configurations or outposts through a central
Kubernetes cluster.

**Caveats:** AWS has better-supported solutions: EKS Anywhere, EKS on Outposts, and SSM
Fleet Manager.

---

## Recommendation

| If your goal is... | Recommendation |
|---|---|
| Run serverless pods on AWS | Use **EKS + Fargate profiles** |
| Dynamic EC2 autoscaling in EKS | Use **Karpenter** |
| Manage macOS builds via Kubernetes | This project is **the most relevant option**, but needs modernization |
| Build a custom VK provider | Consider **interLink** first, use this as reference |
| Run non-container workloads in K8s | Evaluate whether K8s orchestration is worth the complexity vs. Terraform/SSM |
| HPC burst from Kubernetes | Look at **interLink** or **Kueue** |

### Bottom line

The project occupies a narrow but real gap: **Kubernetes-native management of
non-containerizable EC2 workloads, particularly macOS.** No AWS-native service fills this
gap today. However, the cost of modernizing the codebase is significant, and the target
audience is small. For most teams, the answer is to use EKS+Fargate or EKS+Karpenter
instead.

If you have the specific need for macOS or bare-metal EC2 management through Kubernetes,
this project is a reasonable starting point -- fork it, modernize the dependencies, and
adapt the VKVMA agent to your workload. Otherwise, it has served its purpose and been
superseded by native AWS integrations.
