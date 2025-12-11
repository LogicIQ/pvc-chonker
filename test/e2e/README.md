# E2E Tests

End-to-end tests using Kind (Kubernetes in Docker) with real kubelet metrics and storage.

## What's Tested

- Real PVC expansion with actual storage usage
- Kubelet volume metrics integration
- Cooldown period enforcement
- Complete operator workflow

## Prerequisites

```bash
go install sigs.k8s.io/kind@latest
# kubectl and docker required
```

## Usage

```bash
# Setup cluster
task e2e-setup

# Run tests
task e2e-run

# Cleanup
task e2e-cleanup
```

## Direct Commands

```bash
cd test/e2e

# Setup and run
task setup
task run
task cleanup
```

## Test Environment

- Kind cluster with local-path storage
- Real kubelet metrics
- Actual PVC expansion
- Production-like operator deployment