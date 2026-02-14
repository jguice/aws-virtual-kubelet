# AWS Virtual Kubelet — Product Roadmap

> **Status**: Pre-production / Proof-of-Concept
> **Overall Production Readiness**: ~3.5/10
> **Last Updated**: 2026-02-14

---

## Table of Contents

- [Executive Summary](#executive-summary)
- [Use Cases & Business Opportunities](#use-cases--business-opportunities)
- [Current State Assessment](#current-state-assessment)
- [Roadmap Phases](#roadmap-phases)
- [Architecture Evolution](#architecture-evolution)
- [Revenue Model Ideas](#revenue-model-ideas)
- [Risk Register](#risk-register)

---

## Executive Summary

AWS Virtual Kubelet bridges Kubernetes orchestration with EC2 bare-metal instances — enabling workloads that **cannot run in containers** (macOS, GPU-native, bare-metal) to be managed through standard `kubectl` workflows. The project has solid architectural foundations (Virtual Kubelet framework, gRPC agent protocol, warm pool design) but requires significant hardening before production use.

**The key insight**: This project sidesteps the "no GPU in containers" problem on macOS. Because it provisions **full EC2 instances as pods**, workloads get native Metal GPU access — something Docker/Podman on macOS cannot provide.

**Biggest opportunities**: iOS/macOS CI/CD-as-a-Service, LLM inference on Apple Silicon, and creative/media processing pipelines.

---

## Use Cases & Business Opportunities

### Tier 1 — Proven, High-Demand Use Cases

#### 1. iOS / macOS CI/CD-as-a-Service

**The problem**: Every company building iOS apps needs macOS build infrastructure. Most teams cobble together Jenkins on Mac Minis, or pay premium prices for cloud CI (GitHub Actions macOS runners: $0.08/min vs $0.008/min for Linux — **10x cost premium**).

**What we enable**:
- Kubernetes-native iOS build pipelines on EC2 Mac instances
- Auto-scaling Xcode build farms managed by standard K8s tooling
- Warm pool pre-boots Mac instances (45-min boot time eliminated)
- Teams use familiar K8s manifests, Helm charts, ArgoCD — no new tooling

**Target customers**:
- Mobile-first companies (fintech, social, gaming, e-commerce)
- Enterprises with large iOS app portfolios (banks, airlines, retailers)
- CI/CD platform providers (extend their Mac offerings)

**Revenue potential**: High — iOS CI is a multi-billion dollar market segment. Companies like MacStadium, Codemagic, and Bitrise charge $100-500+/month per Mac build agent.

**Competitive advantage**: K8s-native (no proprietary platform), warm pool (fast boot), cost optimization via shared EC2 Dedicated Hosts.

---

#### 2. macOS Application Testing at Scale

**The problem**: macOS app testing (UI testing, integration testing, QA automation) requires real macOS environments. Virtualization is limited (Apple's license restricts VM count), and simulators don't catch all bugs.

**What we enable**:
- Spin up/tear down macOS test environments as K8s Jobs
- Parallel test execution across multiple Mac instances
- Consistent, reproducible test environments via AMI snapshots
- Integration with standard K8s test frameworks (Tekton, Argo Workflows)

**Target customers**:
- macOS app developers (Electron, SwiftUI, AppKit)
- QA-as-a-Service providers
- Enterprises with internal macOS tooling

**Revenue potential**: Medium-High — especially bundled with CI/CD.

---

#### 3. LLM Inference on Apple Silicon

**The problem**: Running large language models requires expensive multi-GPU NVIDIA setups. An A100 80GB costs ~$2/hr on AWS; you need 2-4 for a 70B model. Apple Silicon M2 Ultra has **192GB unified memory** for ~$6.80/hr (mac2-m2ultra.metal) — enough to load a 70B+ model in a single node.

**What we enable**:
- K8s-orchestrated LLM inference fleet on Apple Silicon
- Metal-accelerated inference via `llama.cpp`, `MLX`, or `CoreML`
- Auto-scaling inference endpoints based on request load
- Warm pool for instant model serving (no cold start)
- Unified memory means no GPU↔CPU transfer bottleneck

**Target customers**:
- AI startups needing cost-effective inference
- Enterprises running private LLMs (compliance/privacy)
- MLOps teams already using K8s for ML pipelines

**Revenue potential**: Very High — LLM inference infrastructure is the fastest-growing cloud segment. Apple Silicon offers a unique price/performance niche for memory-bound models.

**Key numbers**:
| Setup | Memory | Cost/hr | 70B Model? |
|-------|--------|---------|------------|
| 1x A100 80GB | 80GB VRAM | ~$2.00 | No (needs 2+) |
| 2x A100 80GB | 160GB VRAM | ~$4.00 | Yes |
| 1x M2 Ultra | 192GB unified | ~$6.80 | Yes, single node |
| 4x A10G 24GB | 96GB VRAM | ~$5.00 | Barely (quantized) |

The Apple Silicon sweet spot: **models too large for one GPU but too small to justify multi-GPU clusters**.

---

### Tier 2 — Strong Niche Use Cases

#### 4. Creative / Media Processing Pipelines

**The problem**: Video transcoding, image processing, and 3D rendering benefit enormously from hardware acceleration. Metal provides hardware H.265/ProRes encoding and Metal Performance Shaders for image ops.

**What we enable**:
- K8s batch jobs for video transcoding (ProRes, H.265 via Metal)
- Image processing pipelines using Metal Performance Shaders
- 3D rendering farms using Metal-accelerated renderers
- Integration with media asset management systems

**Target customers**:
- Media companies, studios, post-production houses
- UGC platforms (TikTok-style video processing)
- Advertising / creative agencies

**Revenue potential**: Medium — niche but high-value per customer.

---

#### 5. Apple Platform Security Research & Testing

**The problem**: Security researchers and enterprises need real macOS/iOS environments for vulnerability testing, malware analysis, and compliance verification. Containers can't run macOS.

**What we enable**:
- Ephemeral macOS environments for security testing
- Automated vulnerability scanning pipelines
- iOS app security analysis (runtime instrumentation)
- Compliance verification (MDM, endpoint security)

**Target customers**:
- Security research firms
- Enterprise security teams
- Government / defense contractors
- App store security review automation

**Revenue potential**: Medium — high willingness-to-pay in security.

---

#### 6. Game Development & Testing

**The problem**: Game studios need to test on real Apple hardware (Metal rendering, Game Center integration, performance profiling). Cross-platform games need macOS/iOS build + test infrastructure.

**What we enable**:
- Metal shader compilation and testing pipelines
- Performance benchmarking on real Apple GPU hardware
- Automated screenshot / gameplay testing
- Cross-platform build farms (Mac + Linux + Windows via different node types)

**Target customers**:
- Game studios (indie to AAA)
- Cross-platform game engine companies
- Game QA services

**Revenue potential**: Medium.

---

#### 7. Desktop Application Distribution & Signing

**The problem**: macOS apps require code signing, notarization, and stapling — all of which must run on macOS. This is a bottleneck in many release pipelines.

**What we enable**:
- Automated signing and notarization pipelines as K8s Jobs
- Secure keychain management on ephemeral instances
- Parallel notarization for large app portfolios
- Integration with release management tools (Fastlane, etc.)

**Target customers**:
- ISVs shipping macOS apps
- Enterprise IT (internal macOS tool distribution)
- Open-source projects with macOS builds

**Revenue potential**: Medium-Low standalone, High when bundled with CI/CD.

---

### Tier 3 — Emerging / Speculative Use Cases

#### 8. CoreML Model Training & Fine-Tuning

**The problem**: Training/fine-tuning models specifically for Apple Neural Engine deployment is best done on Apple hardware, where you can validate performance characteristics during training.

**What we enable**:
- K8s-managed training jobs on Apple Silicon
- Train → optimize → deploy loop on same hardware
- CoreML model conversion and validation pipelines
- Neural Engine performance benchmarking

**Target customers**: ML teams shipping on-device Apple models.
**Revenue potential**: Low-Medium (niche but growing).

---

#### 9. AR/VR Content Pipeline (Vision Pro)

**The problem**: Apple Vision Pro content creation requires macOS-based Reality Composer Pro, Metal rendering, and spatial computing SDKs. No Linux equivalents exist.

**What we enable**:
- Automated 3D model optimization for Vision Pro
- Spatial video / MR content processing pipelines
- Reality Composer Pro headless builds
- Automated testing of visionOS apps

**Target customers**: Vision Pro content creators, spatial computing startups.
**Revenue potential**: Low now, potentially High as Vision Pro adoption grows.

---

#### 10. Non-Mac: Specialized EC2 Workloads via K8s

**The problem**: Some workloads need bare-metal EC2 (FPGA, Graviton, Inferentia, Trainium) but teams want K8s orchestration.

**What we enable**:
- K8s pods backed by any EC2 instance type
- FPGA development and deployment (f1 instances)
- AWS Inferentia/Trainium for ML inference/training
- Graviton ARM workloads without container overhead
- Bare-metal networking (DPDK, SR-IOV) via K8s

**Target customers**: HPC, fintech (low-latency trading), genomics, ML teams.
**Revenue potential**: Medium — broad applicability.

---

#### 11. Managed Dev Environments (Cloud Desktops)

**The problem**: Remote macOS development environments are hard to provision and manage. Solutions like AWS WorkSpaces don't offer macOS.

**What we enable**:
- On-demand macOS development environments as K8s pods
- Pre-configured dev environments via AMIs (Xcode, tools pre-installed)
- Auto-shutdown idle environments (cost savings)
- SSH/VNC access managed through K8s services

**Target customers**: Enterprises with remote developers, dev environment platform teams.
**Revenue potential**: Medium.

---

#### 12. Regulatory & Compliance Sandboxes

**The problem**: Financial services, healthcare, and government need isolated compute environments for data processing, with audit trails and strict lifecycle management.

**What we enable**:
- Ephemeral, auditable compute environments
- K8s-managed lifecycle (auto-terminate, no data persistence)
- EC2 tag-based tracking for compliance reporting
- Integration with AWS CloudTrail, Config, GuardDuty

**Target customers**: Banks, healthcare providers, government agencies.
**Revenue potential**: Medium-High (high regulatory willingness-to-pay).

---

### Use Case Priority Matrix

| # | Use Case | Demand | Revenue | Effort | Priority |
|---|----------|--------|---------|--------|----------|
| 1 | iOS/macOS CI/CD | Very High | High | Medium | **P0** |
| 2 | macOS Test at Scale | High | Medium-High | Medium | **P0** |
| 3 | LLM Inference on Apple Silicon | High | Very High | Medium | **P0** |
| 4 | Creative/Media Pipelines | Medium | Medium | Low | **P1** |
| 5 | Security Research | Medium | Medium | Low | **P1** |
| 6 | Game Dev & Testing | Medium | Medium | Low | **P1** |
| 7 | App Signing & Notarization | Medium | Low-Med | Low | **P2** |
| 8 | CoreML Training | Low-Med | Low-Med | Medium | **P2** |
| 9 | Vision Pro Content | Low (growing) | High (future) | High | **P3** |
| 10 | Specialized EC2 (FPGA, etc.) | Medium | Medium | Low | **P1** |
| 11 | Managed Dev Environments | Medium | Medium | High | **P2** |
| 12 | Compliance Sandboxes | Low-Med | Med-High | Medium | **P2** |

---

## Current State Assessment

### Production Readiness Scorecard

| Area | Score | Status |
|------|-------|--------|
| Test Coverage | 2/10 | ~10% coverage; core provider + warm pool untested |
| CI/CD Pipeline | 4/10 | Basic (fmt, vet, test); no security scanning or releases |
| Documentation | 6/10 | Good architecture docs; missing ops runbook |
| Code Quality | 5/10 | Decent structure; 39 TODOs, mixed error patterns |
| Security | 3/10 | Insecure by default (plaintext gRPC, broad IAM) |
| Observability | 6/10 | 50+ Prometheus metrics; no tracing |
| Error Handling | 3/10 | Infinite retries possible; no circuit breaker |
| Configuration | 5/10 | Flexible; some hardcoded values |
| Deployment | 4/10 | Basic CDK + manifests; no Helm chart |
| Mac/Metal Support | 6/10 | Designed for it; warm pool exists but untested |
| Agent (VKVMA) | 1/10 | Proof-of-concept only |
| Warm Pool | 3/10 | Implemented but zero test coverage |
| Dependencies | 2/10 | Go 1.14, K8s v0.19 — significantly outdated |
| **OVERALL** | **3.5/10** | **Not production ready** |

### Critical Blockers

1. **Agent is example-only** — returns hardcoded success; no real workload execution
2. **No meaningful test coverage** — core provider and warm pool have zero tests
3. **Insecure by default** — gRPC without TLS, overly broad IAM policies
4. **Outdated dependencies** — Go 1.14, K8s client v0.19 (current: Go 1.22+, K8s 1.29+)
5. **Infinite retry loops** — no circuit breakers, no max retry limits
6. **No pod persistence** — in-memory only; VK restart orphans EC2 instances

---

## Roadmap Phases

### Phase 0 — Foundation Stabilization (Weeks 1–4)

> **Goal**: Make the codebase trustworthy enough to build on.

- [ ] **Update Go to 1.22+** and modernize `go.mod`
- [ ] **Update K8s client libraries** to v0.29+
- [ ] **Update AWS SDK** to latest v2
- [ ] **Update Virtual Kubelet** framework to latest
- [ ] **Update all transitive dependencies** and run vulnerability scan
- [ ] **Add `golangci-lint` to CI** with strict config
- [ ] **Add `gosec` to CI** for security static analysis
- [ ] **Add dependency vulnerability scanning** (Dependabot / Trivy)
- [ ] **Resolve all 39 TODOs** or convert to tracked issues
- [ ] **Fix known bugs** (e.g., vkvmagent success/error inversion)

**Exit Criteria**: Clean build on modern Go, all CI checks green, zero known bugs.

---

### Phase 1 — Test Coverage & Core Hardening (Weeks 3–8)

> **Goal**: Achieve 70%+ test coverage on critical paths.

- [ ] **Write unit tests for `ec2provider.go`** (CreatePod, DeletePod, GetPod, UpdatePod, GetPodStatus)
- [ ] **Write unit tests for `warmpool.go`** (all state transitions, edge cases)
- [ ] **Write unit tests for `podcache.go`**
- [ ] **Write unit tests for `compute.go`** (EC2 lifecycle)
- [ ] **Write integration tests** using AWS SDK mocks
- [ ] **Write tests for `vkvmaclient`** (gRPC client)
- [ ] **Write tests for `awsutils`** (EC2/S3 operations)
- [ ] **Implement circuit breaker pattern** for EC2 API calls
- [ ] **Add max retry limits** with configurable backoff
- [ ] **Add resource cleanup finalizers** (no orphaned EC2 instances)
- [ ] **Implement graceful shutdown** with connection draining
- [ ] **Set up coverage threshold enforcement** in CI (fail if <70%)

**Exit Criteria**: 70%+ coverage, no infinite retry paths, clean shutdown.

---

### Phase 2 — Security Hardening (Weeks 6–10)

> **Goal**: Secure by default, least-privilege, auditable.

- [ ] **Enable mTLS by default** for VK ↔ Agent communication
- [ ] **Implement certificate rotation** (automatic renewal before expiry)
- [ ] **Narrow IAM policies** to tag-based resource restrictions
- [ ] **Enforce IMDSv2** on all provisioned EC2 instances
- [ ] **Add EBS encryption** by default
- [ ] **Implement K8s NetworkPolicies** for VK pods
- [ ] **Add PodSecurity standards** (restricted profile)
- [ ] **Add secrets scanning** to CI (gitleaks or similar)
- [ ] **Create security hardening guide** (documentation)
- [ ] **Implement audit logging** (CloudTrail integration, K8s Events)
- [ ] **Add RBAC** with least-privilege for VK service account

**Exit Criteria**: TLS everywhere, no `*` in IAM policies, security scanning in CI.

---

### Phase 3 — Production Agent (Weeks 8–14)

> **Goal**: A real, production-grade workload agent for macOS.

- [ ] **Design agent workload execution model**
  - Option A: systemd-managed processes (simpler, macOS-native)
  - Option B: Apple Virtualization.framework VMs (isolated, but complex)
  - Option C: Process-level isolation with sandbox-exec (macOS native)
- [ ] **Implement LaunchApplication** with real process execution
- [ ] **Implement TerminateApplication** with graceful SIGTERM → SIGKILL
- [ ] **Implement health checks** based on actual process state
- [ ] **Add stdout/stderr log streaming** (new gRPC endpoint)
- [ ] **Add resource monitoring** (CPU, memory, disk via `host_processor_info`)
- [ ] **Implement bootstrap service** (TLS cert exchange, identity verification)
- [ ] **Add environment variable injection** from K8s pod spec
- [ ] **Add volume mount support** (at minimum: ConfigMap, Secret as files)
- [ ] **Implement lifecycle hooks** (preStop, postStart)
- [ ] **Add exec support** (interactive shell into running workload)
- [ ] **Create Mac-specific AMI build pipeline** (Packer)
  - Pre-install Xcode, common tools
  - VKVMA agent pre-installed and auto-starting
  - Metal drivers verified working

**Exit Criteria**: Agent can launch real macOS workloads, stream logs, report real resource usage.

---

### Phase 4 — Pod Persistence & Reliability (Weeks 12–16)

> **Goal**: Survive VK restarts without orphaning resources.

- [ ] **Implement Pod Persistence RFC**
  - Tag EC2 instances with pod metadata
  - On startup, query EC2 tags to rebuild pod cache
  - Handle bare pods correctly
- [ ] **Implement Unrecoverable Failures RFC**
  - Max retry counts per operation
  - Exponential backoff with jitter
  - Dead letter / failed pod reporting
- [ ] **Add warm pool state persistence** (survive VK restart)
- [ ] **Handle EC2 maintenance events** (scheduled retirement, stop)
- [ ] **Implement pod eviction** on node pressure or instance issues
- [ ] **Add leader election** for multi-replica VK deployments
- [ ] **Implement pod migration** on VK scale-down (drain support)
- [ ] **Add orphaned resource cleanup** (periodic reconciliation loop)

**Exit Criteria**: VK can crash and restart without losing pods or leaking EC2 instances.

---

### Phase 5 — Observability & Operations (Weeks 14–18)

> **Goal**: Production teams can monitor, debug, and operate confidently.

- [ ] **Integrate OpenTelemetry** for distributed tracing
  - Trace: K8s API → VK → EC2 API → Agent gRPC
- [ ] **Add gRPC latency histograms** to existing metrics
- [ ] **Add pod lifecycle event recording** (K8s Events API)
- [ ] **Add resource usage metrics** from agent (CPU/mem/disk/GPU)
- [ ] **Create Grafana dashboards** (pod lifecycle, warm pool, errors, costs)
- [ ] **Create alerting rules** (Prometheus/CloudWatch)
  - Orphaned instances
  - Warm pool depletion
  - Agent unreachable
  - Certificate expiry
  - Error rate spikes
- [ ] **Create operational runbook**
  - Troubleshooting guide
  - Common failure scenarios and remediation
  - Scaling procedures
  - Disaster recovery steps
- [ ] **Add pprof endpoint** for runtime profiling
- [ ] **Implement structured JSON logging** option

**Exit Criteria**: Full visibility into system health, actionable alerts, ops team can self-serve.

---

### Phase 6 — Deployment & Developer Experience (Weeks 16–20)

> **Goal**: One-command deployment, great developer experience.

- [ ] **Create Helm chart**
  - All configuration via values.yaml
  - Support for multiple VK replicas
  - Optional monitoring stack (Prometheus + Grafana)
  - RBAC, NetworkPolicy, PodSecurity included
- [ ] **Modernize CDK stack**
  - Parameterize all hardcoded values
  - Add monitoring/alerting resources
  - Add Dedicated Host management (for Mac instances)
  - Support multi-region deployment
- [ ] **Create Terraform module** (alternative to CDK)
- [ ] **Build multi-arch Docker images** (amd64, arm64)
- [ ] **Create Mac AMI build pipeline** (Packer + CI)
- [ ] **Add automated release process**
  - Semantic versioning
  - Changelog generation
  - Container image publishing (ECR Public / Docker Hub)
  - Helm chart publishing
- [ ] **Create quickstart guide** (5-minute deploy)
- [ ] **Create example deployments**
  - iOS CI/CD pipeline (Xcode build + test)
  - LLM inference endpoint (llama.cpp + Metal)
  - Video transcoding job (ffmpeg + Metal)

**Exit Criteria**: `helm install` to running cluster in <10 minutes.

---

### Phase 7 — Mac Metal Optimizations (Weeks 18–24)

> **Goal**: First-class Apple Silicon / Metal experience.

- [ ] **Dedicated Host lifecycle management**
  - Auto-allocate/release Dedicated Hosts
  - Host affinity for warm pool instances
  - Cost optimization (pack instances onto hosts)
- [ ] **Metal GPU resource scheduling**
  - Report Metal GPU as K8s extended resource
  - Schedule GPU-requiring pods to Metal-capable nodes
  - Track GPU utilization metrics
- [ ] **Apple Silicon instance type support matrix**
  - mac2.metal (M1)
  - mac2-m2.metal (M2)
  - mac2-m2pro.metal (M2 Pro)
  - mac2-m2ultra.metal (M2 Ultra) — priority for LLM inference
- [ ] **Warm pool per instance type**
  - Different pools for different instance types
  - Smart routing: CI jobs → M2 Pro, LLM inference → M2 Ultra
- [ ] **macOS version management**
  - Support multiple macOS versions via AMI selection
  - Xcode version pinning per pod annotation
- [ ] **Performance benchmarking suite**
  - Metal compute benchmarks
  - LLM inference throughput (tokens/sec)
  - Xcode build time comparisons
  - Publish results for marketing

**Exit Criteria**: Optimized, measured, documented Mac Metal experience.

---

### Phase 8 — Scale, Multi-Tenancy & Commercialization (Weeks 22–30)

> **Goal**: Ready for multi-tenant SaaS or enterprise deployment.

- [ ] **Multi-tenancy support**
  - Namespace-based tenant isolation
  - Per-tenant resource quotas
  - Per-tenant warm pools
  - Cost attribution via EC2 tags
- [ ] **Horizontal scaling**
  - Multiple VK instances per cluster
  - Sharded pod management
  - Leader election and work distribution
- [ ] **Cost optimization features**
  - Spot instance support (for non-Mac workloads)
  - Auto-shutdown idle instances
  - Right-sizing recommendations
  - Cost dashboards per namespace/team
- [ ] **API rate limiting & throttling**
  - AWS API call budgets
  - Per-tenant request limits
- [ ] **SLA monitoring**
  - Pod startup latency tracking (P50, P95, P99)
  - Availability tracking
  - Error budget dashboards
- [ ] **Enterprise features**
  - SSO / OIDC integration for agent auth
  - Compliance reporting (SOC2, HIPAA artifacts)
  - Backup & disaster recovery automation
  - Cross-region failover

**Exit Criteria**: Multi-tenant production deployment serving paying customers.

---

## Architecture Evolution

### Current Architecture (v0.5.x)
```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  K8s API    │────▶│  Virtual Kubelet │────▶│  EC2 API    │
│  (kubectl)  │     │  + EC2 Provider  │     │  (launch)   │
└─────────────┘     └──────────────────┘     └──────┬──────┘
                            │                        │
                            │ gRPC (plaintext)       │
                            ▼                        ▼
                    ┌──────────────────┐     ┌─────────────┐
                    │  Example Agent   │     │ EC2 Instance│
                    │  (hardcoded OK)  │◀───▶│ (any type)  │
                    └──────────────────┘     └─────────────┘
```

### Target Architecture (v1.0)
```
┌─────────────┐     ┌──────────────────────────────────────┐
│  K8s API    │────▶│  Virtual Kubelet + EC2 Provider      │
│  (kubectl)  │     │  ┌────────────┐  ┌────────────────┐  │
│  (Helm)     │     │  │ Circuit    │  │ Pod Persistence│  │
│  (ArgoCD)   │     │  │ Breaker    │  │ (EC2 Tags)     │  │
└─────────────┘     │  └────────────┘  └────────────────┘  │
                    │  ┌────────────┐  ┌────────────────┐  │
                    │  │ Warm Pool  │  │ Leader Election│  │
                    │  │ (per-type) │  │ (multi-replica)│  │
                    │  └────────────┘  └────────────────┘  │
                    └──────────┬───────────────────────────┘
                               │ mTLS (cert rotation)
                               │ OpenTelemetry traces
                    ┌──────────▼───────────────────────────┐
                    │  Production Agent (VKVMA)             │
                    │  ┌─────────┐  ┌───────────────────┐  │
                    │  │ Process │  │ Log Streaming     │  │
                    │  │ Manager │  │ (stdout/stderr)   │  │
                    │  └─────────┘  └───────────────────┘  │
                    │  ┌─────────┐  ┌───────────────────┐  │
                    │  │ Resource│  │ Health Reporter   │  │
                    │  │ Monitor │  │ (real process)    │  │
                    │  └─────────┘  └───────────────────┘  │
                    └──────────────────────────────────────┘
                               │
                    ┌──────────▼───────────────────────────┐
                    │  EC2 Mac Instance (Apple Silicon)     │
                    │  ┌──────┐ ┌───────┐ ┌─────────────┐ │
                    │  │Metal │ │Xcode  │ │ llama.cpp / │ │
                    │  │ GPU  │ │builds │ │ MLX / CoreML│ │
                    │  └──────┘ └───────┘ └─────────────┘ │
                    └──────────────────────────────────────┘
                               │
                    ┌──────────▼───────────────────────────┐
                    │  Observability Stack                  │
                    │  Prometheus + Grafana + OTel + Alerts │
                    └──────────────────────────────────────┘
```

---

## Revenue Model Ideas

*For later business case analysis.*

### Model A: Managed Service (SaaS)
- Host the full stack; customers deploy pods to your Mac fleet
- Pricing: per-pod-minute (like GitHub Actions pricing)
- Margin: spread across Dedicated Host utilization
- Example: $0.15/min for Mac M2 Pro build agent vs GitHub's $0.16/min

### Model B: Enterprise License
- Sell the hardened, supported version of this project
- Pricing: per-node annual license ($X,000/node/year)
- Include: support, security patches, Helm charts, monitoring
- Target: enterprises already running EKS

### Model C: Open Core
- Open-source the base Virtual Kubelet provider
- Sell premium features: multi-tenancy, compliance dashboards, warm pool management UI, SLA monitoring
- Pricing: per-cluster or per-namespace subscription

### Model D: Consulting / Professional Services
- Help enterprises set up Mac K8s infrastructure
- Build custom agents for specific workloads
- Ongoing managed operations

### Model E: Marketplace (AWS)
- Publish as AWS Marketplace AMI + Helm chart
- Pay-as-you-go via AWS billing
- Leverage AWS co-sell programs

### Rough Unit Economics (Model A — iOS CI/CD)

| Item | Cost | Notes |
|------|------|-------|
| EC2 mac2-m2pro.metal | ~$0.065/min | On-demand pricing |
| Dedicated Host overhead | ~$0.010/min | Amortized |
| Infrastructure (EKS, etc.) | ~$0.005/min | Shared |
| **Total cost** | **~$0.08/min** | |
| **Sell price** | **$0.14-0.18/min** | Competitive with GH Actions |
| **Gross margin** | **~43-55%** | |

At 100 concurrent build agents averaging 50% utilization:
- Monthly revenue: ~$300K-400K
- Monthly cost: ~$175K-225K
- Monthly gross profit: ~$125K-175K

---

## Risk Register

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Apple changes EC2 Mac licensing | High | Low | Diversify to self-hosted Mac hardware |
| AWS deprecates Mac instance types | High | Very Low | Multi-cloud support (Azure, GCP) |
| Virtual Kubelet framework abandoned | Medium | Low | Fork and maintain; evaluate alternatives |
| Security breach via agent | Critical | Medium | mTLS, least-privilege, audit logging |
| Warm pool cost overrun | Medium | Medium | Auto-shutdown, utilization monitoring |
| EC2 Mac capacity constraints | High | Medium | Multi-region, reserved capacity |
| Competition from GitHub/GitLab | High | High | Differentiate on K8s-native, flexibility |
| Apple Silicon perf advantage eroded | Medium | Medium | Stay multi-instance-type, not Apple-only |

---

## Timeline Summary

```
Month 1-2:   Phase 0 + 1  — Foundation + Tests          ████████░░░░░░░░░░░░
Month 2-3:   Phase 2      — Security Hardening           ░░░░████████░░░░░░░░
Month 3-5:   Phase 3 + 4  — Agent + Persistence          ░░░░░░░░████████████
Month 5-6:   Phase 5      — Observability                ░░░░░░░░░░░░████████
Month 5-7:   Phase 6      — Deployment + DX              ░░░░░░░░░░████████░░
Month 6-8:   Phase 7      — Mac Metal Optimizations      ░░░░░░░░░░░░████████
Month 7-10:  Phase 8      — Scale + Commercialization    ░░░░░░░░░░░░░░██████

MVP (Phases 0-4):  ~4-5 months
Production (0-6):  ~6-7 months
Commercial (0-8):  ~8-10 months
```

---

*This roadmap is a living document. Priorities should be revisited monthly based on customer feedback, market signals, and technical discoveries.*
