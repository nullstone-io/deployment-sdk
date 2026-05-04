# Kubernetes Deployment Failure Modes

Catalog deliverable for [NUL-26](https://linear.app/nullstone/issue/NUL-26/enumerate-kubernetes-deployment-failure-modes-and-improve-deployment).
This document is the **Phase 1** enumeration. Phase 2 (regression tests) and Phase 3 (log-pipeline changes) consume it.

Scope: generic Kubernetes plus provider-specific modes on **GKE**, **EKS**, **AKS**.

## How to read an entry

Each entry follows this schema (from NUL-26):

- **Manifestation** — concrete k8s signals: event `reason`, pod phase/conditions, container state reasons, Deployment/ReplicaSet conditions, status fields.
- **Root cause** — what the user did wrong.
- **Detection heuristic** — precise API fields / event-message substrings a watcher should match.
- **Report to user** — canonical name, remediation hint, offending object identifiers.
- **Docs** — authoritative links.
- **Scope** — generic / GKE / EKS / AKS.

Provider scope tags: `generic`, `gke`, `eks`, `aks`. Multiple tags allowed.

---

## Current Nullstone coverage (baseline)

Reference points for where this catalog plugs into today's code. Verify before building on these claims — file/line references drift.

| Already detected | Location |
|---|---|
| Deployment `ProgressDeadlineExceeded` timeout | `check_deployment.go` — `DeploymentProgressing` condition with `TimedOutReason` |
| Deployment deleted mid-watch | `deploy_watcher.go` |
| Superseding revision (higher `.metadata.generation`) | `deploy_watcher.go` |
| Pod terminal phase (`PodFailed`, `PodSucceeded`) stops log stream | `logs/workload_streamer.go` |
| Incomplete rollout (`updatedReplicas` / `availableReplicas` < desired) | `check_deployment.go` |
| Raw namespace event stream (all `Reason`s forwarded as-is) | `deploy_watcher.go` |
| Service endpoint ready/not-ready transitions | `service_watcher.go` |

**Structural gaps the catalog must close:**

1. Container-level `state.waiting.reason` / `state.terminated.reason` are received in `ContainerStatus` but never parsed into a canonical failure name. Users see raw events, not a diagnosis.
2. Probe failures are forwarded as generic `Unhealthy` events without being labeled as readiness / liveness / startup.
3. `Deployment.status.conditions[type=ReplicaFailure]` is not surfaced — quota, admission-webhook, and PSA denials are silent until the progress deadline fires.
4. No provider-specific discriminators on event messages (EKS/GKE/AKS). Provider deployers (`aws/eks`, `gcp/gke`, `azure/aks`) all share the generic `k8s.Deployer` and add nothing beyond kubeconfig/auth.
5. No cross-object checks (referenced ConfigMap/Secret/SA/PVC existence, IngressClass presence, PodSecurity namespace labels).

---

## Detection strategy (top-down)

A deployment watcher should layer checks in this order. The catalog entries below slot into this pipeline.

1. **Deployment-level signal**
   - `status.conditions[type=Progressing,status=False,reason=ProgressDeadlineExceeded]` → rollout failed; drill down.
   - `status.conditions[type=ReplicaFailure,status=True]` → child create blocked (quota / admission / RBAC / PSA). Read condition message.
   - `availableReplicas < spec.replicas` past deadline → degraded.
2. **ReplicaSet-level** (newest RS, highest `deployment.kubernetes.io/revision`)
   - Events with `reason=FailedCreate` carry admission / quota details.
3. **Pod-level triage** (pods owned by newest RS)
   - `status.phase` = `Pending` → check `conditions[type=PodScheduled]`:
     - `False, reason=Unschedulable` → **Scheduling** bucket (parse event message).
     - `False, reason=SchedulingGated` → external gate controller hasn't cleared.
     - `True` but `Initialized=False` → **Volumes / init containers**.
     - `Initialized=True` but `ContainersReady=False` → **Image / probes**.
   - `containerStatuses[].state.waiting.reason` drives the bucket.
   - `containerStatuses[].lastState.terminated.{reason,exitCode}` classifies CrashLoopBackOff causes.
   - `status.reason=Evicted` → node pressure; parse message.
4. **Event bus** per pod — `kubectl get events --field-selector involvedObject.kind=Pod,involvedObject.name=X`. Bucket by `reason`; classify with `message` substrings.
5. **Cross-object validation** (optional, disambiguates ambiguous signals)
   - Referenced ConfigMap / Secret / SA / PVC / IngressClass exist.
   - ServiceAccount carries expected annotations (IRSA / WI / ACR pull).
   - PodSecurity namespace labels vs pod spec.
6. **Provider discriminators** on event messages — see §8–§10.

### Quick cheat sheet

| Signal | Bucket |
|---|---|
| `state.waiting.reason` ∈ {`ImagePullBackOff`, `ErrImagePull`, `InvalidImageName`, `ErrImageNeverPull`, `ImageInspectError`} | Image (§1) |
| `state.waiting.reason=CrashLoopBackOff` + `lastState.terminated.reason=OOMKilled` | OOM (§2.2) |
| `state.waiting.reason=CrashLoopBackOff` + other exitCode | App crash (§2.1) |
| `state.waiting.reason=CreateContainerConfigError` | Config missing (§2.4) |
| `state.waiting.reason` ∈ {`CreateContainerError`, `RunContainerError`} | Runtime config (§2.5–2.6) |
| `conditions[PodScheduled]=False, reason=Unschedulable` | Scheduling (§3) |
| `conditions[PodScheduled]=False, reason=SchedulingGated` | Scheduling gate (§3.6) |
| Event `FailedMount` / `FailedAttachVolume` | Storage (§4) |
| Event `FailedCreatePodSandBox` | CNI / IPAM (§5.1) |
| Event `Unhealthy` | Probes (§2.3) |
| Event `BackOff` | Restart loop (pair with `lastState`) |
| `status.reason=Evicted` | Node pressure (§11.2) |
| Event `Preempted` | Priority preemption (§11.3) |
| Deployment `ReplicaFailure=True` | Quota / admission / webhook / PSA (§6) |
| Deployment `Progressing=False, reason=ProgressDeadlineExceeded` | Top-level rollout timeout (§7.1) |
| PDB `disruptionsAllowed=0` during rollout | PDB block (§3.5) |
| `readinessGates[].status != True` | External gate (§7.8) |

---

## 1. Image / Registry

### 1.1 ImagePullBackOff / ErrImagePull

- **Manifestation**: Pod `Pending`. `containerStatuses[].state.waiting.reason` cycles `ErrImagePull` → `ImagePullBackOff`. Events: `reason=Failed`, message contains one of:
  - `unauthorized`, `authentication required`, `denied` — auth failure.
  - `manifest unknown`, `not found`, `repository ... not found` — missing tag/repo.
  - `toomanyrequests`, `rate limit` — Docker Hub throttling.
  - `no such host`, `i/o timeout`, `connection refused` — network/DNS.
- **Root cause**: wrong tag/digest; private registry without `imagePullSecrets`; expired creds; registry unreachable; anonymous-pull rate limit.
- **Detection**: `state.waiting.reason ∈ {"ErrImagePull","ImagePullBackOff"}`. Subclassify via event message.
- **Report**: `ImagePullBackOff` with sub-reason (`auth`, `not-found`, `rate-limit`, `network`). Include `namespace/pod`, container name, image reference, and the offending event message.
- **Remediation**: verify the tag exists in the registry; attach a valid `imagePullSecret`; for Docker Hub, authenticate or use a mirror.
- **Docs**: <https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff>, <https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/>.
- **Scope**: `generic` (with provider-specific variants in §8.x, §9.6, §10.2).

### 1.2 InvalidImageName

- **Manifestation**: `state.waiting.reason=InvalidImageName`. Event `reason=InspectFailed` or `Failed`, message `couldn't parse image reference`.
- **Root cause**: malformed `spec.containers[].image` — stray whitespace, unexpanded `${TAG}`, uppercase in registry host.
- **Detection**: `state.waiting.reason == "InvalidImageName"`.
- **Report**: offending image string, container name.
- **Remediation**: fix the image reference.
- **Scope**: `generic`.

### 1.3 ErrImageNeverPull / ImageInspectError

- **Manifestation**: `state.waiting.reason ∈ {"ErrImageNeverPull","ImageInspectError"}`. `ErrImageNeverPull` appears when `imagePullPolicy=Never` and the image isn't preloaded on the node.
- **Root cause**: bad `imagePullPolicy`; node-local image corruption.
- **Detection**: direct reason match.
- **Remediation**: set `imagePullPolicy: IfNotPresent`, or preload the image. GKE Autopilot denies `Never`.
- **Scope**: `generic`.

### 1.4 Image architecture mismatch (amd64 / arm64)

- **Manifestation**: `ErrImagePull` (manifest lacks the node's arch) **or** container starts then immediately exits — `terminated.reason=Error`, `exitCode ∈ {1, 139, 255}`, logs contain `exec format error`.
- **Root cause**: image built for one arch, scheduled onto a different-arch node (EKS Graviton, GKE T2A, AKS Ampere).
- **Detection**: cross-reference `pod.spec.nodeName` → `node.status.nodeInfo.architecture` vs image manifest arch. The quick heuristic is `exec format error` in logs.
- **Remediation**: publish a multi-arch manifest (`docker buildx --platform linux/amd64,linux/arm64`) or pin `nodeSelector: kubernetes.io/arch: amd64`.
- **Docs**: <https://kubernetes.io/docs/reference/labels-annotations-taints/#kubernetes-io-arch>.
- **Scope**: `generic` (prevalence: `eks` Graviton, `gke` T2A, `aks` Ampere).

### 1.5 Image pull secret not propagated to ServiceAccount

- **Manifestation**: same as §1.1 (auth branch). Works under `default` SA, fails after switching to a custom SA that lacks `imagePullSecrets`.
- **Detection**: Pod's SA `imagePullSecrets` is empty while image is private.
- **Report**: point at the ServiceAccount, not the pod.
- **Scope**: `generic`.

---

## 2. Runtime / Container Lifecycle

### 2.1 CrashLoopBackOff

- **Manifestation**: `state.waiting.reason=CrashLoopBackOff`; `lastState.terminated.exitCode != 0`; event `reason=BackOff`, message `Back-off restarting failed container`.
- **Root cause**: bad entrypoint/args, missing config, unhandled panic on startup, dependency unreachable, port conflict.
- **Detection**: `state.waiting.reason=="CrashLoopBackOff"`. Subclassify via `lastState.terminated.{reason,exitCode}` and previous-container logs. Exit codes: `137`=SIGKILL (often OOM precursor), `139`=SIGSEGV, `143`=SIGTERM.
- **Report**: container name, `exitCode`, last N lines of `kubectl logs --previous`, restart count.
- **Remediation**: fetch previous-container logs; fix the startup path.
- **Docs**: <https://kubernetes.io/docs/tasks/debug/debug-application/debug-pods/>.
- **Scope**: `generic`.

### 2.2 OOMKilled

- **Manifestation**: `lastState.terminated.reason=OOMKilled`, `exitCode=137`. Often followed by `CrashLoopBackOff`. Container-scoped; not node-level pressure.
- **Root cause**: exceeded `resources.limits.memory`; memory leak; runtime heap not sized to the limit (JVM / .NET / Node).
- **Detection**: `containerStatuses[].lastState.terminated.reason == "OOMKilled"`.
- **Report**: container, current memory limit, recommend inspecting metrics.
- **Remediation**: raise `limits.memory`; fix the leak; size the runtime heap (`-XX:MaxRAMPercentage`, `GOMEMLIMIT`).
- **Docs**: <https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/>.
- **Scope**: `generic`.

### 2.3 Liveness / Readiness / Startup probe failures

- **Manifestation**:
  - **Liveness** fail → event `reason=Unhealthy` + `reason=Killing`, message `Liveness probe failed: ...`; container restarts, usually → CrashLoopBackOff.
  - **Readiness** fail → `pod.conditions[type=Ready,status=False]`; Endpoints excludes the pod; Deployment `Available` stays false.
  - **Startup** fail → container killed after `failureThreshold * periodSeconds`; same `Unhealthy` event with probe-type=Startup.
- **Detection**:
  - Event `reason=Unhealthy` with message prefix `(Liveness|Readiness|Startup) probe failed`.
  - `containerStatuses[].ready=false` with `restartCount > 0`.
- **Report**: probe type, endpoint (`httpGet.path` / `port`, `exec.command`, `tcpSocket.port`), failure count, last error.
- **Remediation**: add a `startupProbe`; widen `initialDelaySeconds` / `failureThreshold`; verify the endpoint.
- **Docs**: <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>.
- **Scope**: `generic`.

### 2.4 CreateContainerConfigError

- **Manifestation**: `state.waiting.reason=CreateContainerConfigError`. Event `reason=Failed`, message `couldn't find key X in ConfigMap Y` or `secret "X" not found`.
- **Root cause**: Pod references a missing ConfigMap/Secret or missing key; invalid `subPath`; broken `envFrom`.
- **Detection**: reason match; parse event message for `configmap|secret|key`.
- **Report**: name the missing object/key.
- **Remediation**: create the object/key, or mark `optional: true`.
- **Scope**: `generic`.

### 2.5 CreateContainerError

- **Manifestation**: `state.waiting.reason=CreateContainerError`. Runtime-specific event message: `container name "x" already in use`, `invalid mount`, `path not allowed`, `failed to create shim task`.
- **Root cause**: runtime-level rejection — hostPath denied, seccomp profile absent, invalid capability, name collision after crash, bad volume mount.
- **Detection**: reason match; branch on message.
- **Scope**: `generic`.

### 2.6 RunContainerError

- **Manifestation**: `state.waiting.reason=RunContainerError`. Messages like `exec: "/app": permission denied`, `no such file or directory`.
- **Root cause**: entrypoint not executable; missing binary; arch mismatch (overlap with §1.4).
- **Scope**: `generic`.

### 2.7 ContainerStatusUnknown / ContainerCannotRun

- **Manifestation**: `state.terminated.reason=ContainerStatusUnknown` (kubelet lost track after a restart); `state.waiting.reason=ContainerCannotRun`.
- **Scope**: `generic`.

### 2.8 Ephemeral storage eviction

- **Manifestation**: Pod phase `Failed`, `status.reason=Evicted`, `status.message` contains `The node was low on resource: ephemeral-storage`. Event `reason=Evicted`.
- **Root cause**: wrote past `limits.ephemeral-storage`; `emptyDir` bloat; log bloat without rotation; node DiskPressure.
- **Detection**: `pod.status.reason == "Evicted"` + message regex on `ephemeral-storage`.
- **Remediation**: set `limits.ephemeral-storage`; use `emptyDir.sizeLimit`; mount a PVC for large writes.
- **Docs**: <https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/>.
- **Scope**: `generic`.

### 2.9 Job DeadlineExceeded

- **Manifestation**: `job.status.conditions[type=Failed,reason=DeadlineExceeded]`; pod `status.reason=DeadlineExceeded`; event `reason=DeadlineExceeded`.
- **Root cause**: `activeDeadlineSeconds` shorter than job runtime.
- **Docs**: <https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup>.
- **Scope**: `generic`.

### 2.10 Job BackoffLimitExceeded

- **Manifestation**: `job.status.conditions[type=Failed,reason=BackoffLimitExceeded]`; `status.failed >= spec.backoffLimit + 1`.
- **Detection**: condition match; last pod's `terminated.exitCode` carries the real cause.
- **Scope**: `generic`.

### 2.11 Job FailedIndexes / PodFailurePolicy (K8s ≥1.28)

- **Manifestation**: Indexed Jobs — `status.failedIndexes`, `conditions[reason in {FailedIndexes,PodFailurePolicy}]`.
- **Scope**: `generic`.

---

## 3. Scheduling

### 3.1 FailedScheduling — insufficient resources

- **Manifestation**: Pod `Pending`; `conditions[type=PodScheduled,status=False,reason=Unschedulable]`. Event `reason=FailedScheduling`, message `0/N nodes are available: N Insufficient (cpu|memory|ephemeral-storage|pods)`.
- **Detection**: event message regex on `Insufficient (cpu|memory|ephemeral-storage|pods)`.
- **Remediation**: reduce `requests`; add nodes; enable cluster autoscaler.
- **Docs**: <https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/>.
- **Scope**: `generic`.

### 3.2 FailedScheduling — taints not tolerated

- **Detection**: `FailedScheduling` message contains `had untolerated taint {key=value:NoSchedule}` or `node(s) had taint`.
- **Notable taints to recognize and surface**:
  - `node.kubernetes.io/not-ready`, `unreachable` — node degraded.
  - `node.kubernetes.io/unschedulable` — cordoned.
  - **GKE**: `nvidia.com/gpu=present:NoSchedule`, `cloud.google.com/gke-spot=true:NoSchedule`, `components.gke.io/gke-managed-components=true:NoSchedule`.
  - **EKS**: `eks.amazonaws.com/compute-type=fargate:NoSchedule`, `karpenter.sh/disrupted`.
  - **AKS**: `kubernetes.azure.com/scalesetpriority=spot:NoSchedule`, `CriticalAddonsOnly=true:NoSchedule` (system pool).
- **Remediation**: add matching `tolerations`, or target a different node pool.
- **Scope**: `generic` + provider taints (`gke`, `eks`, `aks`).

### 3.3 FailedScheduling — nodeAffinity / nodeSelector / pod (anti-)affinity / topologySpread

- **Detection**: message contains `didn't match Pod's node affinity/selector`, `didn't match pod affinity`, `didn't match pod anti-affinity`, or `didn't match pod topology spread constraints`.
- **Root cause**: node labels missing; required anti-affinity impossible for cluster topology; `maxSkew` too tight.
- **Remediation**: align labels (`topology.kubernetes.io/zone`, `kubernetes.io/arch`, `node.kubernetes.io/instance-type`, `cloud.google.com/gke-nodepool`, `eks.amazonaws.com/nodegroup`, `agentpool`); relax to `preferredDuringScheduling`.
- **Scope**: `generic`.

### 3.4 FailedScheduling — autoscaler won't scale up

- **Manifestation**: `0/0 nodes are available`, or autoscaler event `NotTriggerScaleUp` on the pod with reasons like `pod didn't trigger scale-up: N node(s) didn't match ...`.
- **Detection**:
  - **GKE CA**: events on pod from source `cluster-autoscaler` with `NotTriggerScaleUp`.
  - **EKS Karpenter**: pod events from source `karpenter`; `NodeClaim.status.conditions[type=Launched,status=False,reason=InsufficientCapacityError]`.
  - **AKS CA**: same `NotTriggerScaleUp` event.
- **Scope**: `generic` heuristic with provider-specific event sources.

### 3.5 PodDisruptionBudget blocking rollout

- **Manifestation**: Eviction API returns 429 during rollout/drain; Deployment hangs at N-1 of N.
- **Detection**: `PodDisruptionBudget.status.disruptionsAllowed == 0` while `expectedPods > currentHealthy`. 429s on `Eviction` via controller logs or audit.
- **Remediation**: raise `maxUnavailable` (or lower `minAvailable`) on the PDB; scale replicas up first; wait for healthy pods.
- **Docs**: <https://kubernetes.io/docs/tasks/run-application/configure-pdb/>.
- **Scope**: `generic`.

### 3.6 Scheduling gates (K8s ≥1.27)

- **Manifestation**: `spec.schedulingGates[].name` present; `conditions[type=PodScheduled,status=False,reason=SchedulingGated]`.
- **Root cause**: external controller hasn't cleared the gate (Karpenter consolidation, Kueue).
- **Scope**: `generic`.

### 3.7 FailedScheduling — too many pods / max-pods cap

- **Detection**: message `too many pods` or `Insufficient pods`. Kubelet `--max-pods` (default 110, provider-overridden; EKS default tied to ENI capacity per instance type).
- **Scope**: `generic`; provider defaults differ (see §9.4).

### 3.8 hostPort collisions

- **Detection**: `FailedScheduling` message `node(s) didn't have free ports for the requested pod ports`.
- **Scope**: `generic`.

### 3.9 Extended resources not advertised

- **Detection**: `FailedScheduling ... Insufficient nvidia.com/gpu` (or equivalent).
- **Remediation**: install the device plugin / add a node pool that advertises the resource.
- **Scope**: `generic`.

---

## 4. Storage

### 4.1 PVC Pending — no StorageClass / provisioning failure

- **Manifestation**: `pvc.status.phase=Pending`. Events: `reason ∈ {ProvisioningFailed,FailedBinding,ExternalProvisioning}`, messages like `no persistent volumes available for this claim and no storage class is set`, `failed to provision volume with StorageClass "X"`.
- **Root cause**: missing/misnamed `storageClassName`; cloud-disk quota; zone mismatch with `volumeBindingMode=Immediate`.
- **Detection**: `pvc.status.phase=="Pending"` past ~30s + event reason match.
- **Remediation**: set correct StorageClass; prefer `volumeBindingMode: WaitForFirstConsumer` for zonal volumes.
- **Docs**: <https://kubernetes.io/docs/concepts/storage/persistent-volumes/>.
- **Scope**: `generic`.

### 4.2 FailedMount / FailedAttachVolume

- **Manifestation**: Pod stuck `ContainerCreating`. Event `reason ∈ {FailedMount,FailedAttachVolume}`. Common messages:
  - `Unable to attach or mount volumes: ... timed out waiting for the condition` — slow/stuck CSI.
  - `Multi-Attach error for volume` — RWO volume still attached to a previous node.
  - `does not exist` — PV/disk deleted out of band.
  - `rpc error: code = ... CSI` — CSI driver-specific.
- **Root cause**: CSI crash; multi-attach after pod rescheduling; bad fsType; cloud IAM missing; backend network; filesystem repair needed.
- **Detection**: event reason match; branch on message.
- **Scope**: `generic` (with provider variants §4.3).

### 4.3 FailedAttachVolume — cloud credentials / quotas

Provider discriminators on the event message:

- **EKS**: `UnauthorizedOperation`, `VolumeLimitExceeded` (EBS caps 39–128 per instance by type), `IncorrectState`.
- **GKE**: `googleapi: Error 403: ... requires one of ["compute.instances.attachDisk"]`, `QUOTA_EXCEEDED`.
- **AKS**: `Code="OperationNotAllowed"`, `Code="DiskAttachmentFailed"`, `AttachVolume.Attach failed ... cannot attach data disk ... lun`.

### 4.4 CSI driver not installed

- **Manifestation**: PVC stuck; events `waiting for a volume to be created, either by external provisioner "ebs.csi.aws.com" or manually created by system administrator`.
- **Detection**: `StorageClass.provisioner` has no matching running CSI controller / no `CSIDriver` object / no `CSINode` advertising it.
- **Scope**: `eks` (EBS CSI is an addon since 1.23), `gke` (PD CSI), `aks` (Azure Disk/File CSI, default).

### 4.5 StatefulSet ordinal stuck / PVC pending

- **Manifestation**: `sts.status.readyReplicas < spec.replicas`; pod `web-2` stuck `Pending` because PVC `data-web-2` won't bind (zone pin, quota).
- **Detection**: PVC owned by StatefulSet, phase `Pending`; StatefulSet `currentReplicas` stalled.
- **Remediation**: align zone topology; raise disk quota; use `WaitForFirstConsumer`.
- **Scope**: `generic`.

### 4.6 Read-only filesystem / permission denied

- **Manifestation**: container crashes writing to mount; logs `read-only file system` or `permission denied`.
- **Root cause**: `readOnlyRootFilesystem: true` without a writable volume; fsGroup mismatch; CSI doesn't support fsGroup.
- **Detection**: combine `securityContext.readOnlyRootFilesystem` with crash logs. Hard to detect generically — surface via probe failures + log keyword.
- **Scope**: `generic`.

### 4.7 VolumeSnapshot / online-resize failures

- **Manifestation**: PVC condition `FileSystemResizePending`; event `VolumeResizeFailed`.
- **Scope**: `generic`.

---

## 5. Network

### 5.1 FailedCreatePodSandBox — CNI / IPAM

- **Manifestation**: Pod stuck `ContainerCreating`. Event `reason=FailedCreatePodSandBox`. Message branches:
  - `no IP addresses available in range set` / `InsufficientFreeAddressesInSubnet` — subnet IP exhaustion.
  - `failed to assign an IP address to container` — IPAM error.
  - `context deadline exceeded` — CNI daemon unhealthy.
- **Detection**: reason match; branch on message.
- **Provider discriminators**:
  - **EKS VPC CNI**: `InsufficientFreeAddressesInSubnet`, `unable to attach ENI`, `AttachmentLimitExceeded`. Mitigations: prefix delegation, `WARM_ENI_TARGET`, larger subnets, custom networking.
  - **GKE**: `IP_SPACE_EXHAUSTED` on Pod CIDR / secondary range. Add secondary ranges or enable "Discover additional Pod IP address ranges".
  - **AKS Azure CNI (classic)**: `SubnetIsFull`, `Failed to allocate address`. Migrate to Azure CNI Overlay or shrink per-node CIDR.
  - **AKS kubenet**: route-table cap (400 routes).
- **Docs**:
  - EKS: <https://docs.aws.amazon.com/eks/latest/userguide/pod-networking.html>.
  - GKE: <https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips>.
  - AKS: <https://learn.microsoft.com/azure/aks/configure-azure-cni>.
- **Scope**: `generic` event, provider-specific diagnostics.

### 5.2 NetworkPolicy blocking traffic

- **Manifestation**: no direct event — symptom is readiness probe failure (if probe uses pod-to-pod) or app-level connection errors. Kubelet probes originate on the node and still succeed.
- **Detection**: correlate readiness failures with a NetworkPolicy selecting the pod that lacks a matching ingress/egress allow rule. Requires introspection, not an event match.
- **Remediation**: add an explicit allow rule; test with `kubectl run -it --rm netshoot`.
- **Docs**: <https://kubernetes.io/docs/concepts/services-networking/network-policies/>.
- **Scope**: `generic`.

### 5.3 Service type=LoadBalancer provisioning failure

- **Manifestation**: `svc.status.loadBalancer.ingress` never populated. Events on Service: `reason ∈ {SyncLoadBalancerFailed, EnsuringLoadBalancer, CreatingLoadBalancerFailed}`.
- **Detection**: `spec.type=="LoadBalancer"` AND `len(status.loadBalancer.ingress)==0` past the provisioning SLO, plus Service events.
- **Provider discriminators on message**:
  - **EKS** (in-tree / AWS LB Controller): `TooManyLoadBalancers`, `AccessDenied` (missing IAM), `no matching subnets` (missing `kubernetes.io/role/elb` or `kubernetes.io/role/internal-elb` subnet tags), `SecurityGroupLimitExceeded`.
  - **GKE**: `googleapi: Error 403 ... forwardingRules.create`, `QUOTA_EXCEEDED`, `BackendService not ready`. NEG sync errors from `neg-controller`.
  - **AKS**: `OutboundRuleCannotBeUsedWithLoadBalancerSku`, `PublicIPCountLimitReached`, `LoadBalancerInUseByVirtualMachineScaleSet`, `AuthorizationFailed` (missing role on `MC_` resource group).
- **Docs**:
  - EKS: <https://kubernetes-sigs.github.io/aws-load-balancer-controller/>.
  - GKE: <https://cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress>.
  - AKS: <https://learn.microsoft.com/azure/aks/load-balancer-standard>.
- **Scope**: provider-specific (`eks`, `gke`, `aks`).

### 5.4 Ingress controller / IngressClass misconfig

- **Manifestation**: Ingress created but no address, or path routes 404. Events from the controller (or none, if no controller is installed).
- **Detection**:
  - `ingress.spec.ingressClassName` points to a nonexistent `IngressClass` → no controller.
  - **GKE ingress-gce**: events `Sync`, `LoadBalancerSync`, `BackendConfig not found`, `neg not ready for target`.
  - **AWS ALB Controller**: mismatch between legacy `kubernetes.io/ingress.class` annotation and `ingressClassName`; `FailedBuildModel`.
  - **AKS AGIC / nginx**: controller pod logs.
- **Scope**: provider-specific variants.

### 5.5 TLS cert / Secret mismatch in Ingress

- **Manifestation**: `ingress.spec.tls[].secretName` references a missing or malformed Secret (not `kubernetes.io/tls` type; cert CN/SAN mismatch).
- **Detection**: missing Secret → controller emits `FailedGet`; cert-manager `Certificate.Ready=False, reason∈{Issuing,Failed}`; browser serves default cert.
- **Remediation**: provision via cert-manager or upload valid `tls.crt`/`tls.key`; ensure SAN covers the host.
- **Scope**: `generic` (with GKE Managed Certs and AKS App Gateway variants).

### 5.6 CoreDNS / service discovery failure

- **Manifestation**: apps can't resolve hostnames; HTTP-name probes fail.
- **Detection**: readiness failures + CoreDNS pods `Ready=False` or elevated error rate.
- **Scope**: `generic`.

### 5.7 externalTrafficPolicy=Local with no local endpoint

- **Manifestation**: Service type=LoadBalancer/NodePort with `externalTrafficPolicy=Local` — cloud LB health checks fail on nodes without a ready pod; LB marks targets unhealthy; app reports 5xx despite being "deployed".
- **Detection**: `spec.externalTrafficPolicy=="Local"` + pod is absent from a node that the LB health-checks.
- **Scope**: `generic`.

### 5.8 Dual-stack / ipFamilyPolicy mismatch

- **Manifestation**: Service create rejected when `ipFamilyPolicy` or `ipFamilies` don't match cluster configuration.
- **Scope**: `generic`.

---

## 6. Admission / Validation

### 6.1 Invalid manifest (apply-time)

- **Manifestation**: `kubectl apply` (or controller create) returns HTTP 4xx. No pod is created.
- **Examples**: schema violation; immutable-field change (Deployment `selector`, Service `clusterIP`, StatefulSet `volumeClaimTemplates`); `PVC.spec.resources.requests.storage` shrink.
- **Detection**: apply-time error, not a runtime event. For Nullstone, this surfaces as an apply error before any watcher starts.
- **Scope**: `generic`.

### 6.2 Admission webhook rejection (validating / mutating)

- **Manifestation**: create returns `admission webhook "x.example.com" denied the request: <reason>`. For Deployments this manifests as `Deployment.conditions[type=ReplicaFailure,status=True,reason=FailedCreate]` with the webhook message; event on the ReplicaSet `reason=FailedCreate`.
- **Detection**:
  - Deployment condition: `status=True, type=ReplicaFailure` with message containing `admission webhook ... denied`.
  - RS events: `reason=FailedCreate` with webhook name in message.
- **Common offenders**: OPA/Gatekeeper, Kyverno, cert-manager, service meshes (Istio/Linkerd injectors), provider policy controllers (GKE Policy Controller, Azure Policy).
- **Remediation**: check the webhook's policies/logs; if the webhook is down, inspect `failurePolicy`.
- **Docs**: <https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/>.
- **Scope**: `generic`.

### 6.3 Pod Security Admission (PSA) denial

- **Manifestation**: create rejected `pods "x" is forbidden: violates PodSecurity "restricted:v1.N": <control list>`. Deployment surfaces it as `ReplicaFailure`.
- **Detection**: condition/event message contains `violates PodSecurity`.
- **Root cause**: namespace labeled `pod-security.kubernetes.io/enforce=restricted|baseline` and pod spec violates (privileged, hostPath, `runAsNonRoot=false`, caps).
- **Remediation**: drop caps; `runAsNonRoot: true`; `seccompProfile: RuntimeDefault`; remove hostPath; or relax the namespace label.
- **Docs**: <https://kubernetes.io/docs/concepts/security/pod-security-admission/>.
- **Scope**: `generic` (GKE Autopilot enforces `restricted`+; EKS/AKS opt-in).

### 6.4 RBAC / ServiceAccount denial (in-cluster API calls)

- **Manifestation**: pod runs; controller/operator logs `forbidden: User "system:serviceaccount:ns:sa" cannot ... at the cluster scope`. In-cluster API calls receive 403. No standard pod event.
- **Detection**: app/operator status; audit log 403 trail.
- **Remediation**: create appropriate `Role`/`ClusterRole` + `RoleBinding`/`ClusterRoleBinding`.
- **Scope**: `generic`.

### 6.5 ResourceQuota denial

- **Manifestation**: Pod creation blocked → `Deployment.conditions[type=ReplicaFailure,reason=FailedCreate]` with message `exceeded quota: compute-resources, requested: ..., used: ..., limited: ...`. Event on the RS.
- **Detection**: condition/event match + message prefix `pods "x" is forbidden: exceeded quota`.
- **Remediation**: raise the ResourceQuota; reduce requests; set defaults via LimitRange.
- **Docs**: <https://kubernetes.io/docs/concepts/policy/resource-quotas/>.
- **Scope**: `generic`.

### 6.6 LimitRange denial or silent defaulting

- **Manifestation**: apply rejected with `maximum cpu usage per Container is X, but limit is Y`; or requests/limits silently defaulted → unexpected OOM.
- **Detection**: admission error text contains `LimitRange` or `minimum|maximum ... per Container`.
- **Scope**: `generic`.

### 6.7 SecurityContext denial beyond PSA (OPA, Kyverno, Autopilot)

See §6.2 plus §8.1 for GKE Autopilot.

### 6.8 Namespace Terminating / missing

- **Manifestation**: create fails with `unable to create new content in namespace X because it is being terminated`.
- **Detection**: error text match; `namespace.status.phase=="Terminating"` with stuck finalizers.
- **Scope**: `generic`.

### 6.9 Large Secret / ConfigMap

- **Manifestation**: etcd write limit (~1 MiB) rejects apply.
- **Scope**: `generic`.

---

## 7. Rollout / Controller

### 7.1 Deployment ProgressDeadlineExceeded

- **Manifestation**: `deployment.status.conditions[type=Progressing,status=False,reason=ProgressDeadlineExceeded]`; message `Deployment "x" exceeded its progress deadline`. Fires after `spec.progressDeadlineSeconds` (default 600s) without forward progress.
- **Detection**: condition match is the authoritative top-level signal. Drill down into the newest RS's pods for the underlying reason.
- **Docs**: <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#failed-deployment>.
- **Scope**: `generic`. Nullstone already detects this in `check_deployment.go` — enrichment should come from §1–§6.

### 7.2 Deployment ReplicaFailure

- **Manifestation**: `deployment.status.conditions[type=ReplicaFailure,status=True]` with `reason=FailedCreate` (or similar). Points to a ReplicaSet blocked from creating pods.
- **Common causes**: quota (§6.5), admission webhook (§6.2), PSA (§6.3), missing SA / bad imagePullSecrets, invalid toleration.
- **Detection**: condition present; mirror to newest RS's events.
- **Scope**: `generic`. **Gap in current Nullstone** — not surfaced today.

### 7.3 Rollout stuck on maxSurge / maxUnavailable math

- **Manifestation**: `updatedReplicas < spec.replicas`; `availableReplicas` stuck at exactly `replicas - maxUnavailable`; new pods never reach Ready.
- **Root cause**: new pods fail probes AND `maxUnavailable=0`; or `maxSurge=0` with no cluster capacity for one more.
- **Detection**: newest-RS `availableReplicas` stuck; old-RS `replicas` at `spec.replicas - maxUnavailable`; newest-RS pods show probe failures.
- **Remediation**: fix the probe; temporarily raise `maxSurge` when cluster capacity is the bottleneck.
- **Scope**: `generic`.

### 7.4 minReadySeconds hold

- **Manifestation**: pod `Ready=True` but not counted as available for `minReadySeconds`. Briefly looks frozen.
- **Detection**: `pod.conditions[type=Ready].lastTransitionTime` within the window.
- **Scope**: `generic`.

### 7.5 Rollout blocked by PDB

See §3.5. Deployment `Progressing` can still be `True` while new pods can't evict old ones. Visible via `Eviction` 429s.

### 7.6 HPA churn during rollout

- **Manifestation**: HPA writes `spec.replicas` mid-rollout; evicted pods flash `Preempted` / `FailedScaleIn`.
- **Detection**: HPA `status.currentReplicas` oscillating; writes to `Deployment.spec.replicas` during the rollout window.
- **Remediation**: tune `behavior.scaleDown.stabilizationWindowSeconds`; pause HPA during deploys.
- **Scope**: `generic`.

### 7.7 VPA eviction mid-rollout

- **Manifestation**: pods evicted (VPA admission rewrites requests); events `reason=EvictedByVPA` (or similar).
- **Scope**: `generic` (GKE VPA is native; EKS/AKS require install).

### 7.8 Readiness gate never satisfied

- **Manifestation**: `pod.status.conditions[type=<gate>]` stays `False` or absent; `Ready=False` indefinitely even though container probes pass; Deployment never becomes Available.
- **Common gates**:
  - `target-health.elbv2.k8s.aws/<ingress>_<svc>_<port>` — AWS Load Balancer Controller.
  - Service-mesh readiness gates.
- **Detection**: `pod.spec.readinessGates[]` combined with `pod.status.conditions[type=<gate>].status != "True"`.
- **Remediation**: diagnose the external system the gate represents — ALB target health: security group, health-check path, subnet.
- **Docs**: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-gate>.
- **Scope**: `generic` (most common on `eks`).

### 7.9 Selector mismatch / orphaned ReplicaSets

- **Manifestation**: apply error `Deployment.apps "x" is invalid: spec.selector: field is immutable`. Or two RSes with the same selector owned by different Deployments → thrash.
- **Scope**: `generic`.

### 7.10 Stuck Terminating pods (finalizers)

- **Manifestation**: Pod stays `Terminating` with `deletionTimestamp` set; finalizers aren't removed (custom operator finalizer, `kubernetes.io/pv-protection`). Blocks rollout under strict PDBs or `strategy.type=Recreate`.
- **Detection**: `metadata.deletionTimestamp` set AND `metadata.finalizers` non-empty past termination grace.
- **Scope**: `generic`.

### 7.11 Helm pre/post-hook Job failure

- **Manifestation**: Helm rolls back on hook-Job `BackoffLimitExceeded`. Surface the Job pod's crash details.
- **Scope**: `generic` (applies to Nullstone's Helm-based deploys).

---

## 8. Provider-specific — GKE

### 8.1 GKE Autopilot — Warden policy violations

- **Manifestation**: apply rejected with `GKE Warden rejected the request: ... autopilot.gke.io/<policy>`. Common policies:
  - `no-host-namespace` (`hostNetwork` / `hostPID` / `hostIPC`)
  - `no-host-port`
  - `no-host-path` (only a narrow allowlist)
  - `linux-capabilities` (most caps denied)
  - `no-privileged`
  - `workload-separation` (conflicting nodeSelector/tolerations)
  - `no-defaultserviceaccount-token`
  - `no-write-mode-hostpath`
- **Detection**: admission-error message matches `autopilot.gke.io/` or `GKE Warden`.
- **Docs**: <https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview>, <https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-security-constraints>.
- **Scope**: `gke` (Autopilot).

### 8.2 GKE Autopilot — resource constraints and rewriting

- **Manifestation**: Requests auto-raised to Autopilot minimums (≈0.25 vCPU / 0.5 GiB per pod); CPU:memory ratio enforced (1 vCPU : 1–6.5 GiB). Admission message: `Autopilot set default resource requests ...` or rejection for ratio violation.
- **Detection**: admission message contains `Autopilot`.
- **Remediation**: set requests within the allowed ratio; use `cloud.google.com/compute-class` (Balanced/Scale-Out/Accelerator) for different envelopes.
- **Docs**: <https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-resource-requests>.
- **Scope**: `gke` (Autopilot).

### 8.3 GKE — managed namespace restrictions

- **Manifestation**: writes to `kube-system`, `gke-*`, `gmp-system`, `gke-managed-*` rejected.
- **Scope**: `gke` (Autopilot strictly; Standard protects a subset).

### 8.4 GKE Workload Identity misconfig

- **Manifestation**: Google SDK errors at runtime — `failed to find default credentials`, `could not refresh token: ... 403`. Pods run; cloud API calls fail.
- **Detection** (pre-runtime, cross-object):
  - ServiceAccount missing `iam.gke.io/gcp-service-account` annotation.
  - Namespace not enabled for WI (on old clusters).
  - Missing IAM binding `roles/iam.workloadIdentityUser` on the GSA for `serviceAccount:PROJECT.svc.id.goog[NS/KSA]`.
- **Docs**: <https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity>.
- **Scope**: `gke`.

### 8.5 GKE Alias IP / secondary range exhaustion

- **Manifestation**: `FailedCreatePodSandBox` with `IP_SPACE_EXHAUSTED`; node creation may fail.
- **Remediation**: add secondary ranges; enable "Discover additional Pod IP address ranges"; shrink per-node CIDR.
- **Docs**: <https://cloud.google.com/kubernetes-engine/docs/how-to/flexible-pod-cidr>.
- **Scope**: `gke`.

### 8.6 GKE Ingress (GCLB) / NEG sync errors

- **Manifestation**: Ingress `status.loadBalancer` empty. Events from source `loadbalancer-controller` or `neg-controller`: `Translate`, `Sync`, `BackendConfig "x" not found`, `neg not ready for target`.
- **Detection**: event source + keyword match (`BackendConfig`, `FrontendConfig`, `neg`).
- **Scope**: `gke`.

### 8.7 GKE node-pool taints / special hardware

- Examples: `nvidia.com/gpu`, `cloud.google.com/gke-preemptible`, `cloud.google.com/gke-spot`, `cloud.google.com/gke-tpu`. Pods without matching tolerations → `FailedScheduling`.
- **Scope**: `gke`.

### 8.8 GKE private cluster — webhook / registry reachability

- **Manifestation**: external admission webhooks time out (`failed calling webhook ... context deadline exceeded`); non-GCR image pulls fail (`i/o timeout`) without Cloud NAT.
- **Scope**: `gke` (private clusters).

---

## 9. Provider-specific — EKS

### 9.1 IRSA misconfig

- **Manifestation**: pods run; AWS SDK calls return `WebIdentityErr: unable to assume role: AccessDenied`, `InvalidIdentityToken`, or SDK `NoCredentialProviders`.
- **Detection** (pre-runtime heuristics):
  - ServiceAccount missing `eks.amazonaws.com/role-arn` annotation.
  - Pod env missing `AWS_ROLE_ARN` / `AWS_WEB_IDENTITY_TOKEN_FILE` (should be auto-injected by the Pod Identity webhook / EKS Pod Identity Agent).
  - IAM role trust policy doesn't federate the cluster OIDC provider, or `sub` condition ≠ `system:serviceaccount:<ns>:<sa>`.
  - Cluster OIDC provider not created in IAM.
- **Docs**: <https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html>.
- **Scope**: `eks`.

### 9.2 EKS Pod Identity misconfig

- **Manifestation**: same SDK-level auth failures as §9.1.
- **Detection**: no `PodIdentityAssociation` for the namespace/SA; `eks-pod-identity-agent` DaemonSet not running.
- **Docs**: <https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html>.
- **Scope**: `eks`.

### 9.3 aws-auth ConfigMap / Access Entries missing

- **Manifestation**: `kubectl` returns `You must be logged in to the server (Unauthorized)`; or node never joins (`NotReady` absent from `kubectl get nodes` while EC2 is running).
- **Detection**: For node join — missing `mapRoles` entry with groups `system:bootstrappers,system:nodes` for the node IAM role. For humans/CI — missing `mapRoles`/`mapUsers`. New clusters use Access Entries instead.
- **Docs**: <https://docs.aws.amazon.com/eks/latest/userguide/grant-k8s-access.html>.
- **Scope**: `eks`.

### 9.4 VPC CNI IP exhaustion / ENI limits

- **Manifestation**: `FailedCreatePodSandBox` with `InsufficientFreeAddressesInSubnet`; or `aws-node` logs `AssignPrivateIpAddresses: Unable to assign, too many addresses assigned`.
- **Remediation**: larger subnets; enable prefix delegation (`ENABLE_PREFIX_DELEGATION=true`); custom networking; smaller node instance types; IPv6.
- **Docs**: <https://docs.aws.amazon.com/eks/latest/userguide/cni-increase-ip-addresses.html>.
- **Scope**: `eks`.

### 9.5 EKS Fargate profile constraints

- **Manifestation**: pods stuck `Pending` with event message `no Fargate profiles match pod` or `Pod not supported on Fargate: ...`.
- **Unsupported on Fargate**: privileged pods, DaemonSets, `hostNetwork`/`hostPath`, EBS (only EFS CSI), Classic LB, GPU.
- **Detection**: event message contains `Fargate`; pod label `eks.amazonaws.com/compute-type=fargate`.
- **Docs**: <https://docs.aws.amazon.com/eks/latest/userguide/fargate.html>.
- **Scope**: `eks` (Fargate).

### 9.6 ECR image-pull authorization

- **Manifestation**: `ImagePullBackOff` with `no basic auth credentials` or `401 Unauthorized` on `*.dkr.ecr.*.amazonaws.com`.
- **Root cause**: node IAM role missing `AmazonEC2ContainerRegistryReadOnly`; cross-account ECR without a repository policy; wrong-region endpoint.
- **Detection**: event message contains `dkr.ecr.` + auth keywords.
- **Scope**: `eks`.

### 9.7 Node bootstrap failures

- **Manifestation**: EC2 running but node absent from API server; or node `NotReady` with kubelet log `Unauthorized`.
- **Causes**: wrong cluster name in userdata; aws-auth missing; security groups not allowing control plane; IMDSv2 hop limit (EKS ≥1.27 nodes).
- **Scope**: `eks`.

### 9.8 Security Groups for Pods

- **Manifestation**: pod has `vpc.amazonaws.com/pod-eni` but stays `ContainerCreating` with ENI errors. Requires Nitro instances; increases per-instance ENI consumption.
- **Scope**: `eks`.

### 9.9 Karpenter — NodeClaim launch failure

- **Manifestation**: pod `Pending` with Karpenter events `FailedLaunch`, `Incompatible`, `InsufficientCapacity`. `NodeClaim.status.conditions[type=Launched,status=False,reason=InsufficientCapacityError]`, message `no instance types satisfied resources ...`.
- **Detection**: watch `NodeClaim.status.conditions`; pod events from source `karpenter`.
- **Docs**: <https://karpenter.sh/docs/troubleshooting/>.
- **Scope**: `eks` (and self-managed).

---

## 10. Provider-specific — AKS

### 10.1 Workload Identity / Pod Identity misconfig

- **Manifestation**: Azure API calls fail with `AADSTS700016` (invalid client), `AADSTS70021` (no matching federated identity credential), `AADSTS7000215` (invalid secret), or `ManagedIdentityCredential authentication failed`.
- **Detection** (pre-runtime):
  - ServiceAccount missing `azure.workload.identity/client-id`.
  - Pod missing label `azure.workload.identity/use: "true"`.
  - No Federated Identity Credential on the UAMI / App Registration linking OIDC issuer + `subject system:serviceaccount:<ns>:<sa>`.
  - AKS cluster OIDC issuer or Workload Identity feature not enabled.
- Legacy AAD Pod Identity (deprecated): missing `AzureIdentity` / `AzureIdentityBinding`; pod not selected by binding.
- **Docs**: <https://learn.microsoft.com/azure/aks/workload-identity-overview>.
- **Scope**: `aks`.

### 10.2 ACR pull authentication

- **Manifestation**: `ImagePullBackOff` with `unauthorized: authentication required` on `*.azurecr.io`.
- **Root cause**: kubelet managed identity lacks `AcrPull` on the ACR; `--attach-acr` not run; private ACR without private endpoint from the node subnet; expired `imagePullSecrets`.
- **Detection**: event registry host `.azurecr.io` + auth keywords.
- **Remediation**: `az aks update -n X -g Y --attach-acr <acr>`; grant `AcrPull` to the kubelet identity.
- **Docs**: <https://learn.microsoft.com/azure/aks/cluster-container-registry-integration>.
- **Scope**: `aks`.

### 10.3 Azure CNI IP exhaustion / kubenet route cap

- **Manifestation**: `FailedCreatePodSandBox` with `SubnetIsFull`, `Failed to allocate address`, or `Failed to add route`.
- **Root cause**:
  - Azure CNI (classic): every pod consumes a VNet IP; subnet too small.
  - Azure CNI Overlay: pod IPs are internal to the node; watch node-IP consumption.
  - kubenet: route table cap (400 routes, one per node).
- **Remediation**: migrate to Azure CNI Overlay (or Cilium data plane); expand subnet.
- **Docs**: <https://learn.microsoft.com/azure/aks/concepts-network-cni-overview>.
- **Scope**: `aks`.

### 10.4 Azure Policy / Gatekeeper denial

- **Manifestation**: admission webhook denial with policy names like `K8sAzureV2...`, `K8sPSP...`.
- **Detection**: webhook name `gatekeeper-validating-webhook-configuration` with message referencing an `azure-policy` constraint template.
- **Docs**: <https://learn.microsoft.com/azure/governance/policy/concepts/policy-for-kubernetes>.
- **Scope**: `aks`.

### 10.5 Azure Disk attach conflicts

- **Manifestation**: `FailedAttachVolume` with messages like `disk(/subscriptions/.../disks/X) already attached to node(Y)`, `AttachDiskWhileBeingDetached`, `Cannot attach data disk 'X' to VM ... because the disk is already owned by VM 'Y'`.
- **Root cause**: StatefulSet pod rescheduled; VMSS scale-in; out-of-band attach.
- **Detection**: event message substring `already attached` / `owned by VM`.
- **Remediation**: wait for detach, or force-detach via CSI annotation `disk.csi.azure.com/request-detach`.
- **Scope**: `aks`.

### 10.6 LoadBalancer provisioning (Standard SKU)

- **Manifestation**: Service LB stuck with events including `OutboundRuleCannotBeUsedWithLoadBalancerSku`, `PublicIPCountLimitReached`, `LoadBalancerInUseByVirtualMachineScaleSet`, `AuthorizationFailed` (missing role on `MC_` resource group), `SubnetHasNoFreeAddresses` (internal LB).
- **Remediation**: grant `Network Contributor` on the node resource group; request a quota increase.
- **Scope**: `aks`.

### 10.7 System node pool `CriticalAddonsOnly` taint

- **Manifestation**: user workloads `FailedScheduling` with `had untolerated taint {CriticalAddonsOnly: true}`.
- **Remediation**: add a user node pool (recommended) or tolerate the taint (not recommended).
- **Scope**: `aks`.

### 10.8 API server VNet integration / private cluster

- **Manifestation**: admission webhooks time out; image pulls from external registries fail without outbound route.
- **Scope**: `aks` (private clusters).

---

## 11. Node / Infrastructure

### 11.1 Node NotReady

- **Manifestation**: `node.conditions[type=Ready,status∈{False,Unknown}]`; auto-taints `node.kubernetes.io/not-ready` / `unreachable`; pods eventually evicted after `tolerationSeconds=300`.
- **Root causes**: kubelet crash; container-runtime failure; network partition; `/var` disk full; `PLEG is not healthy`.
- **Detection**: node condition + message (often mentions PLEG, `runtime network not ready`).
- **Scope**: `generic`.

### 11.2 Node pressure taints and evictions

- Taints: `node.kubernetes.io/{memory,disk,pid}-pressure`, `network-unavailable`.
- **Manifestation**: pod `status.reason=Evicted`; event `reason=Evicted`; message `The node was low on resource: (memory|ephemeral-storage|nodefs|imagefs|pids)`.
- **Detection**: `pod.status.reason == "Evicted"` + message regex.
- **Docs**: <https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/>.
- **Scope**: `generic`.

### 11.3 Spot / preemptible termination

- **GKE**: node annotation / event `PreemptibleNodeDeletion`; taint `cloud.google.com/gke-spot`.
- **EKS**: Spot interruption notice handled by `aws-node-termination-handler` → pod eviction `reason=TaintManagerEviction` or taint `node.kubernetes.io/unschedulable`.
- **AKS**: scale-set spot eviction; `kubernetes.azure.com/scalesetpriority=spot`.
- **Scope**: provider-specific.

### 11.4 Taint-based eviction for unreachable nodes

- **Manifestation**: pods `Terminating` then recreated elsewhere after `tolerationSeconds` (default 300s) of `NotReady|Unreachable` taint.
- **Scope**: `generic`.

### 11.5 Pod preemption by higher PriorityClass

- **Manifestation**: event `reason=Preempted` with preemptor identified. `status.nominatedNodeName` set on the victim.
- **Scope**: `generic`.

### 11.6 DisruptionTarget condition (K8s ≥1.26)

- **Manifestation**: `pod.status.conditions[type=DisruptionTarget,reason ∈ {PreemptionByScheduler, EvictionByEvictionAPI, DeletionByTaintManager, TerminationByKubelet}]` — structured disruption metadata useful for accurate reporting.
- **Scope**: `generic`.

### 11.7 RuntimeClass not found

- **Manifestation**: `spec.runtimeClassName` references a missing `RuntimeClass`; pod fails to start with `CreateContainerError` message `no runtime for class ... is configured`.
- **Scope**: `generic`.

### 11.8 inotify / fd exhaustion on node

- **Manifestation**: intermittent probe failures; node kernel logs `fs.inotify.max_user_instances`.
- **Scope**: `generic`.

---

## 12. Reporting format (Phase 3 contract)

Every cataloged failure should emit a structured log record with, at minimum, these fields. The wire format feeds Phase 3 log improvements and NUL-27 (Monitoring tab), NUL-29 (executions list), and NUL-30 (Deployment section).

```yaml
failure:
  name: string         # canonical name from this catalog (e.g. "ImagePullBackOff", "PodSecurityDenial")
  category: string     # top-level section: image|runtime|scheduling|storage|network|admission|rollout|provider|node
  summary: string      # one-line root-cause summary
  remediation: string  # one-line user-facing hint
  object:
    kind: string       # Pod | ReplicaSet | Deployment | PVC | Service | Ingress | Job | Node
    namespace: string
    name: string
    container: string  # when applicable
  signals:             # raw evidence for drill-down UIs
    eventReason: string
    eventMessage: string
    waitingReason: string
    terminatedReason: string
    exitCode: int
    condition: string  # e.g. "Progressing=False:ProgressDeadlineExceeded"
  provider: string     # generic | gke | eks | aks
  docs: [url]          # 1+ authoritative links
```

Consumers (Monitoring tab, executions list) filter on `category` + `provider`. The catalog's canonical names (`name`) are the stable contract — do not rename without a migration.

---

## 13. Open questions / follow-ups

Items worth resolving before / during Phase 2:

- **Probe failure labeling**: confirm whether the current event stream reliably carries the probe type in the message on all k8s versions we support. If not, parse from the `Killing` event that follows.
- **Readiness-gate detection**: should we special-case `target-health.elbv2.k8s.aws/*` on EKS, or keep it generic?
- **NetworkPolicy diagnosis**: detection requires introspection that may be too heavy for every deploy; consider making this opt-in or only firing when probes fail without another explanation.
- **Admission-webhook failures**: when `failurePolicy=Fail` and the webhook itself is down, we want to distinguish "webhook rejected" from "webhook unreachable". Event messages differ: `denied the request` vs `failed calling webhook ... context deadline exceeded`.
- **Provider signal plumbing**: the generic `k8s.Deployer` can surface most of this, but `aws/eks`, `gcp/gke`, `azure/aks` will need hooks for provider-specific cross-object checks (IRSA annotation, WI annotation, ACR attach status, subnet tagging).
- **Evicted pods mid-rollout**: categorize under node (§11.2) vs. spot termination (§11.3) vs. PriorityClass preemption (§11.5) — they all carry `Evicted`/`Preempted` reasons but need different remediation hints.
