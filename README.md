# pvc-chonker

> ⚠️ **UNDER ACTIVE DEVELOPMENT** - This project is currently in early development and **NOT READY FOR PRODUCTION USE**. APIs and functionality may change without notice.

A cloud-agnostic Kubernetes operator for automatic PVC expansion with modular cloud provider support.

## Features

- **Cloud Agnostic**: Modular architecture supporting multiple cloud providers
- **No External Dependencies**: Self-contained operation without external databases
- **Annotation-Based**: Configuration through Kubernetes annotations
- **Monitoring First**: Comprehensive metrics and observability
- **Extensible**: Plugin architecture for cloud provider support

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
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```
