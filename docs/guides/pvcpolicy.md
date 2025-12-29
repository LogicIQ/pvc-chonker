# PVCPolicy Configuration

PVCPolicy provides centralized configuration for multiple PVCs using label selectors, eliminating the need to annotate each PVC individually.

## Overview

PVCPolicy allows you to:
- Define expansion policies once and apply to multiple PVCs
- Use label selectors to automatically target PVCs
- Maintain centralized configuration management
- Override policies with individual PVC annotations when needed

## Basic PVCPolicy

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
    increase: "25%"
    maxSize: "1000Gi"
    cooldown: "30m"
```

## PVCPolicy Fields

### Selector
Defines which PVCs the policy applies to:

```yaml
spec:
  selector:
    matchLabels:
      app: postgres
      tier: database
    matchExpressions:
    - key: environment
      operator: In
      values: ["production", "staging"]
```

### Template
Defines the expansion configuration:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `enabled` | bool | Enable auto-expansion | `true` |
| `threshold` | float64 | Storage usage threshold (%) | `85.0` |
| `inodesThreshold` | float64 | Inode usage threshold (%) | `90.0` |
| `increase` | string | Expansion amount | `"25%"` or `"50Gi"` |
| `maxSize` | string | Maximum size limit | `"2000Gi"` |
| `minScaleUp` | string | Minimum expansion amount | `"10Gi"` |
| `cooldown` | string | Cooldown between expansions | `"30m"` |

## Configuration Examples

### Database Workloads

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: database-storage-policy
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
```

### Log Storage

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: log-storage-policy
  namespace: logging
spec:
  selector:
    matchLabels:
      component: logs
  template:
    enabled: true
    threshold: 75.0
    increase: "50%"
    maxSize: "500Gi"
    minScaleUp: "10Gi"
    cooldown: "15m"
```

### Development Environment

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: dev-storage-policy
  namespace: development
spec:
  selector:
    matchLabels:
      environment: development
  template:
    enabled: true
    threshold: 80.0
    increase: "20%"
    maxSize: "100Gi"
    minScaleUp: "5Gi"
    cooldown: "10m"
```

## Using PVCPolicy

### 1. Create the Policy

```bash
kubectl apply -f database-policy.yaml
```

### 2. Label Your PVCs

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: production
  labels:
    workload: database  # Matches policy selector
    app: postgres
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

### 3. Verify Policy Application

```bash
# Check if policy is applied
kubectl describe pvc postgres-data

# Check policy status
kubectl get pvcpolicy database-policy -o yaml
```

## Priority and Overrides

Configuration follows this precedence order:

1. **PVC Annotations** (highest priority)
2. **PVCPolicy** (namespace-scoped)
3. **Global Defaults** (operator flags/env vars)
4. **Built-in Defaults** (fallback)

### Override Example

```yaml
# PVC with policy override
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: special-database
  labels:
    workload: database  # Matches policy
  annotations:
    # Override policy settings
    pvc-chonker.io/threshold: "90%"  # Higher than policy's 85%
    pvc-chonker.io/increase: "10%"   # Lower than policy's 25%
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 200Gi
```

## Multiple Policies

You can have multiple policies in the same namespace:

```yaml
# Policy for databases
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: database-policy
spec:
  selector:
    matchLabels:
      workload: database
  template:
    threshold: 85.0
    increase: "25%"
---
# Policy for cache storage
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: cache-policy
spec:
  selector:
    matchLabels:
      workload: cache
  template:
    threshold: 70.0
    increase: "50%"
```

## Monitoring Policies

### Check Policy Status

```bash
# List all policies
kubectl get pvcpolicy

# Get policy details
kubectl describe pvcpolicy database-policy

# Check which PVCs match a policy
kubectl get pvc -l workload=database
```

### Policy Metrics

Monitor policy effectiveness with Prometheus metrics:

```promql
# PVCs managed by policies
pvcchonker_pvcpolicy_managed_pvcs

# Policy application success rate
rate(pvcchonker_pvcpolicy_applications_total[5m])
```

## Best Practices

1. **Use Descriptive Names**: Choose clear policy names that indicate their purpose
2. **Namespace Scoping**: Keep policies in the same namespace as target PVCs
3. **Label Consistency**: Use consistent labeling strategy across your cluster
4. **Test Selectors**: Verify selectors match intended PVCs before applying
5. **Monitor Usage**: Track which PVCs are managed by policies
6. **Document Policies**: Maintain documentation of your policy strategy

## Troubleshooting

### Policy Not Applied

```bash
# Check if PVC labels match policy selector
kubectl get pvc your-pvc --show-labels

# Verify policy exists in same namespace
kubectl get pvcpolicy -n your-namespace

# Check operator logs
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager
```

### Conflicting Policies

If multiple policies match the same PVC, the operator will log warnings. Ensure selectors are mutually exclusive or use more specific labels.

### Policy Updates

Changes to PVCPolicy are applied to matching PVCs on the next reconciliation cycle. You can force reconciliation by updating PVC labels.

## Related Documentation

- **[Configuration Guide](./configuration.md)** - Complete configuration reference
- **[PVCGroup](./pvcgroup.md)** - Coordinated expansion policies
- **[Examples](../examples/overview.md)** - Real-world policy examples