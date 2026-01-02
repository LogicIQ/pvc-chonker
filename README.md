# pvc-chonker

![PVC Chonker](https://raw.githubusercontent.com/LogicIQ/pvc-chonker/main/docs/images/pvc-chonker.webp)

A cloud-agnostic Kubernetes operator for automatic PVC expansion. Works with any CSI-compatible storage without external dependencies.

🌐 **[Visit Project Page](https://logiciq.ca/pvc-chonker)**

## Features

- **Cloud Agnostic**: Works with any CSI-compatible storage
- **No External Dependencies**: Self-contained operation without external databases
- **Annotation-Based**: Simple configuration through Kubernetes annotations
- **Policy-Based**: Advanced configuration through PVCPolicy custom resources
- **Cooldown Protection**: Prevents rapid successive expansions
- **Resize Safety**: Checks for ongoing resize operations
- **Configurable Defaults**: Global settings via flags/env vars with per-PVC overrides

## Requirements

### Cluster Prerequisites
- **Kubernetes 1.19+**: CSI volume expansion support
- **Kubelet Metrics**: Volume usage statistics endpoint (`/metrics`)
- **CSI Driver**: Storage class with `allowVolumeExpansion: true`
- **RBAC**: Permissions for PVC updates and storage class reads

### Kubelet Metrics Availability
- **Managed Clusters**: Usually enabled by default (EKS, GKE, AKS)
- **Self-Managed**: May require kubelet configuration
- **Verification**: Check `http://node-ip:10255/metrics` for volume stats
- **Alternative Ports**: Some clusters use `:10250` (requires authentication)

> **Note**: The operator requires access to kubelet metrics to monitor PVC usage. Most managed Kubernetes services enable this by default, but self-managed clusters may need additional configuration.

## Installation

### Helm Chart (Recommended)

```bash
helm repo add logiciq https://logiciq.github.io/helm-charts
helm repo update

# Basic installation
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace

# With PVCGroup support (webhook enabled)
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set webhook.enabled=true
```

### Docker

```bash
docker pull logiciq/pvc-chonker:latest
```

### Binary Downloads

Download platform-specific binaries from [GitHub Releases](https://github.com/logicIQ/pvc-chonker/releases):
- Linux (amd64, arm64)
- macOS (amd64, arm64)

### Manual Deployment

```bash
task build
task docker-build
task deploy
```

## Quick Start

### Basic Usage

Annotate your PVC to enable auto-expansion:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
    pvc-chonker.io/increase: "20%"
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: your-expandable-storage-class
  resources:
    requests:
      storage: 10Gi
```

### Integration Testing

```bash
task test:integration  # Run e2e tests 
task test:deploy      # Redeploy during development
task test:cleanup     # Clean up test environment
```



## Development

See [docs/ROADMAP.md](docs/ROADMAP.md) for detailed project roadmap and development phases.

## Monitoring

PVC Chonker exports comprehensive Prometheus metrics for monitoring and alerting. See [docs/METRICS.md](docs/METRICS.md) for detailed metrics documentation including:

- Resizer success/failure counters
- Client performance metrics
- Operational status indicators
- PVC usage and capacity tracking

Metrics are available at `:8080/metrics` by default.

## Usage

Annotate your PVCs to enable auto-expansion:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
    pvc-chonker.io/inodes-threshold: "80%"
    pvc-chonker.io/increase: "10%"
    pvc-chonker.io/max-size: "100Gi"
    pvc-chonker.io/min-scale-up: "1Gi"
    pvc-chonker.io/cooldown: "15m"
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

### Annotation Reference

| Annotation | Description | Default | Example |
|------------|-------------|---------|----------|
| `pvc-chonker.io/enabled` | Enable auto-expansion | `false` | `"true"` |
| `pvc-chonker.io/threshold` | Storage usage threshold | `80%` | `"85%"` |
| `pvc-chonker.io/inodes-threshold` | Inode usage threshold | `80%` | `"90%"` |
| `pvc-chonker.io/increase` | Expansion amount | `10%` | `"20%"` or `"5Gi"` |
| `pvc-chonker.io/max-size` | Maximum size limit | none | `"1000Gi"` |
| `pvc-chonker.io/min-scale-up` | Minimum expansion amount | `1Gi` | `"2Gi"` or `"500Mi"` |
| `pvc-chonker.io/cooldown` | Cooldown between expansions | `15m` | `"30m"` or `"6h"` |

## Configuration Hierarchy

## Configuration Hierarchy

PVC Chonker uses a priority-based configuration system. Settings are applied in the following order (highest to lowest priority):

### 1. PVC Annotations (Highest Priority)
Direct annotations on individual PVCs always override all other settings:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pvc-chonker.io/threshold: "90%"  # Always takes precedence
```

### 2. PVCGroup Template (Medium-High Priority)
When a PVC matches a PVCGroup selector, group template settings apply:

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
spec:
  template:
    threshold: "80%"  # Used if no PVC annotation exists
```

### 3. PVCPolicy Template (Medium Priority)
Namespace-scoped policies for PVCs with matching labels:

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
spec:
  template:
    threshold: 75.0  # Used if no annotation or group setting
```

### 4. Global Configuration (Low Priority)
Operator-level defaults from command flags or environment variables.

### 5. Built-in Defaults (Lowest Priority)
Hardcoded fallback values: `threshold: 80%`, `increase: 10%`, `cooldown: 15m`

**Important**: Only specified fields override lower priorities. Unspecified fields fall through to the next priority level.

Each setting follows this precedence order:
1. **PVC Annotation** (highest priority)
2. **PVCGroup Template** (if PVC matches group selector)
3. **PVCPolicy Template** (if PVC matches policy selector)
4. **Global Flag/Environment Variable** 
5. **Built-in Default** (fallback)

## PVCGroup Configuration

**Explicit Membership**: PVCs must have both `pvc-chonker.io/group=<group-name>` and `pvc-chonker.io/enabled="true"` annotations to be managed by a PVCGroup.

```bash
# Add PVCs to a group
kubectl annotate pvc my-pvc-1 pvc-chonker.io/group=my-group pvc-chonker.io/enabled=true
kubectl annotate pvc my-pvc-2 pvc-chonker.io/group=my-group pvc-chonker.io/enabled=true

# Or use label selectors to annotate multiple PVCs
kubectl annotate pvc -l app=elasticsearch pvc-chonker.io/group=elasticsearch-cluster pvc-chonker.io/enabled=true
```

For coordinated PVC expansion across related volumes, use PVCGroup custom resources:

```yaml
# 1. Create PVCGroup
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-cluster
  namespace: logging
spec:
  template:
    threshold: "80%"
    increase: "20%"
    maxSize: "1000Gi"
    cooldown: "20m"
```

```bash
# 2. Annotate PVCs to join the group
kubectl annotate pvc -l app=elasticsearch \
  pvc-chonker.io/group=elasticsearch-cluster \
  pvc-chonker.io/enabled=true -n logging
```

### PVCGroup Features

- **Group Coordination**: Maintains consistent sizing across related PVCs using largest size policy
- **Template Application**: Webhook automatically applies group settings to matching PVCs
- **Override Support**: Individual PVC annotations always take precedence
- **Explicit Membership**: PVCs must have both `pvc-chonker.io/group` and `pvc-chonker.io/enabled=true` annotations

## PVCPolicy Configuration

For advanced use cases, configure PVCs using PVCPolicy custom resources instead of individual annotations:

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: database-policy
  namespace: production
spec:
  selector:
    matchLabels:
      workload: database
  template:
    enabled: true
    threshold: 85.0
    inodesThreshold: 90.0
    increase: "25%"
    maxSize: "2000Gi"
    minScaleUp: "50Gi"
    cooldown: "30m"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: production
  labels:
    workload: database  # Matches policy selector
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

### PVCPolicy Benefits

- **Centralized Management**: Define policies once, apply to multiple PVCs
- **Label-Based Selection**: Automatically apply to PVCs with matching labels
- **Namespace Scoped**: Policies only affect PVCs in the same namespace
- **Annotation Override**: Individual PVC annotations always take precedence

### PVCPolicy Fields

| Field | Type | Description | Example |
|-------|------|-------------|----------|
| `selector` | LabelSelector | Which PVCs this policy applies to | `matchLabels: {workload: database}` |
| `template.enabled` | bool | Enable auto-expansion | `true` |
| `template.threshold` | float64 | Storage usage threshold | `85.0` |
| `template.inodesThreshold` | float64 | Inode usage threshold | `90.0` |
| `template.increase` | string | Expansion amount | `"25%"` or `"50Gi"` |
| `template.maxSize` | Quantity | Maximum size limit | `"2000Gi"` |
| `template.minScaleUp` | Quantity | Minimum expansion amount | `"50Gi"` |
| `template.cooldown` | Duration | Cooldown between expansions | `"30m"` |

## Safety Features

- **Cooldown Protection**: Prevents expansions during cooldown period
- **Resize Detection**: Skips PVCs that are currently being resized
- **Size Validation**: Respects maximum size limits
- **Minimum Expansion**: Ensures meaningful size increases (min 1Gi)
- **GiB Rounding**: Rounds up to clean storage boundaries

## Inode Support

**Automatic Inode Monitoring**: PVC Chonker monitors both storage and inode usage:

- Triggers expansion when either storage OR inode threshold is reached
- Works with any filesystem that exposes inode metrics via kubelet
- Provides detailed inode usage in logs and Prometheus metrics
- Gracefully handles volumes without inode metrics (storage-only mode)

**Filesystem Considerations**: Inode expansion effectiveness varies by filesystem:

- **ext3/ext4**: Fixed inode count at creation - expansion won't increase inodes
- **XFS**: Dynamic inodes - expansion resolves both storage and inode pressure
- **Btrfs/ZFS**: Dynamic inodes - fully effective for inode pressure

**Important**: PVC Chonker will detect ext3/ext4 filesystems and warn when inode pressure triggers expansion, as the expansion will not resolve the inode issue.

## Monitoring & Alerting

**Essential Monitoring**: Set up alerts for inode usage, especially on ext3/ext4 filesystems:

```promql
# Alert on high inode usage for ext3/ext4 (expansion won't help)
pvcchonker_pvc_inodes_usage_percent > 85

# Alert on inode pressure expansions that won't resolve the issue
increase(pvcchonker_resizer_failed_resize_total{reason="inode_pressure_fixed_fs"}[5m]) > 0
```

**Recommended Actions**:
- Monitor both storage and inode usage in your alerting system
- For ext3/ext4 with high file counts, plan filesystem migration or recreation with higher inode density
- Use XFS for workloads with many small files

## Examples

See the [`examples/`](examples/) directory for sample PVC configurations:

- [`example-pvc.yaml`](examples/example-pvc.yaml) - Database and logs storage examples with different annotation patterns
- [`pvcpolicy-example.yaml`](examples/pvcpolicy-example.yaml) - PVCPolicy configuration examples

### Database Storage Example

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-storage
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "85%"
    pvc-chonker.io/inodes-threshold: "90%"
    pvc-chonker.io/increase: "25%"
    pvc-chonker.io/max-size: "500Gi"
    pvc-chonker.io/min-scale-up: "2Gi"
    pvc-chonker.io/cooldown: "30m"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
  storageClassName: gp3
```
