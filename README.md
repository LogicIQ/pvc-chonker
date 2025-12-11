# pvc-chonker

> ⚠️ **UNDER ACTIVE DEVELOPMENT** - This project is currently in early development and **NOT READY FOR PRODUCTION USE**. APIs and functionality may change without notice.

A cloud-agnostic Kubernetes operator for automatic PVC expansion. Works with any CSI-compatible storage without external dependencies.

## Features

- **Cloud Agnostic**: Works with any CSI-compatible storage
- **No External Dependencies**: Self-contained operation without external databases
- **Annotation-Based**: Simple configuration through Kubernetes annotations
- **Cooldown Protection**: Prevents rapid successive expansions
- **Resize Safety**: Checks for ongoing resize operations
- **Configurable Defaults**: Global settings via flags/env vars with per-PVC overrides

## Quick Start

### Integration Testing

Run integration tests with kind:

```bash
task test:integration
```

Redeploy operator during development:

```bash
task test:deploy
```

Clean up test environment:

```bash
task test:cleanup
```

### Build and Deploy

```bash
# Build the operator
task build

# Build Docker image
task docker-build

# Deploy to cluster
task deploy
```

## Development

See [ROADMAP.md](ROADMAP.md) for detailed project roadmap and development phases.

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
| `pvc-chonker.io/increase` | Expansion amount | `10%` | `"20%"` or `"5Gi"` |
| `pvc-chonker.io/max-size` | Maximum size limit | none | `"1000Gi"` |
| `pvc-chonker.io/min-scale-up` | Minimum expansion amount | `1Gi` | `"2Gi"` or `"500Mi"` |
| `pvc-chonker.io/cooldown` | Cooldown between expansions | `15m` | `"30m"` or `"6h"` |

## Configuration Hierarchy

Each setting follows this precedence order:
1. **PVC Annotation** (highest priority)
2. **Global Flag/Environment Variable** 
3. **Built-in Default** (fallback)

## Safety Features

- **Cooldown Protection**: Prevents expansions during cooldown period
- **Resize Detection**: Skips PVCs that are currently being resized
- **Size Validation**: Respects maximum size limits
- **Minimum Expansion**: Ensures meaningful size increases (min 1Gi)
- **GiB Rounding**: Rounds up to clean storage boundaries

## Examples

See the [`examples/`](examples/) directory for sample PVC configurations:

- [`example-pvc.yaml`](examples/example-pvc.yaml) - Database and logs storage examples with different annotation patterns

### Database Storage Example

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-storage
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "85%"
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
