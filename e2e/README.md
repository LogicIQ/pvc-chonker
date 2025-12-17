# E2E Tests for PVC Chonker

This directory contains end-to-end tests for PVC Chonker using **Minikube + CSI HostPath driver**.

## Quick Start

Run the complete E2E test suite with a single command:

```bash
cd e2e/
task e2e
```

This will:
1. Create Minikube cluster with CSI HostPath driver
2. Setup test environment and storage class
3. Build and deploy the operator
4. Run all E2E tests
5. Report results

## Individual Commands

```bash
# Create cluster only
task create-cluster

# Setup storage environment
task setup

# Deploy operator
task deploy

# Run tests only
task test

# Check status
task status

# Verify metrics
task verify-metrics

# Clean up
task cleanup
```

## Test Coverage

The E2E tests cover:

- **Basic PVC Expansion** - Volume expansion when storage threshold is reached
- **Kubelet Metrics** - Verification of required volume metrics availability
- **Inode Monitoring** - Inode threshold detection and processing
- **Max Size Limits** - Respecting maximum size constraints
- **Cooldown Periods** - Preventing rapid successive expansions
- **Operator Logs** - Proper logging and reconciliation

## Requirements

- **Minikube** - For local Kubernetes cluster
- **Docker** - For container runtime
- **Task** - For running build tasks
- **Go 1.21+** - For running tests

## Test Environment

- **Cluster**: Minikube with Kubernetes v1.28.0
- **Storage**: CSI HostPath driver with volume expansion enabled
- **Metrics**: Real kubelet volume metrics (not mocked)
- **Timeout**: 10 minutes for complete test suite

## Troubleshooting

If tests fail:

1. Check cluster status: `task status`
2. View operator logs: `kubectl logs deployment/controller-manager -n pvc-chonker-system`
3. Verify metrics: `task verify-metrics`
4. Clean up and retry: `task cleanup && task e2e`