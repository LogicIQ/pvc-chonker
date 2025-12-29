# PVCGroup Configuration

PVCGroup enables coordinated expansion across multiple related PVCs, ensuring consistent sizing within a group of volumes.

## Overview

PVCGroup provides:
- **Coordinated Expansion**: Keep related PVCs at consistent sizes
- **Group Policies**: Different strategies for size coordination
- **Automatic Application**: Webhook applies group settings to matching PVCs
- **Template Management**: Centralized configuration for group members

## Prerequisites

**Webhook Required**: PVCGroups require the admission webhook to be enabled:

```bash
helm install pvc-chonker logiciq/pvc-chonker \
  --set webhook.enabled=true \
  -n pvc-chonker-system --create-namespace
```

## Basic PVCGroup

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-cluster
  namespace: logging
spec:
  selector:
    matchLabels:
      app: elasticsearch
      cluster: main
  coordinationPolicy: "largest"
  template:
    enabled: true
    threshold: "80%"
    increase: "20%"
    maxSize: "1000Gi"
    cooldown: "20m"
```

## Coordination Policies

### Largest Policy
All PVCs match the size of the largest PVC in the group:

```yaml
spec:
  coordinationPolicy: "largest"
```

**Use Case**: Ensures no volume is smaller than others (default behavior)

**Example**: If PVCs are 100Gi, 150Gi, 120Gi → all become 150Gi

### Average Policy
All PVCs match the average size of the group:

```yaml
spec:
  coordinationPolicy: "average"
```

**Use Case**: Balanced approach for cost optimization

**Example**: If PVCs are 100Gi, 200Gi, 150Gi → all become 150Gi

### Sum Policy
Distribute total capacity evenly across all PVCs:

```yaml
spec:
  coordinationPolicy: "sum"
```

**Use Case**: Fixed total capacity scenarios

**Example**: Total 450Gi across 3 PVCs → each becomes 150Gi

## Configuration Examples

### Elasticsearch Cluster

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-data
  namespace: logging
spec:
  selector:
    matchLabels:
      app: elasticsearch
      component: data
  coordinationPolicy: "largest"
  template:
    enabled: true
    threshold: 80.0
    inodesThreshold: 85.0
    increase: "25%"
    maxSize: "2000Gi"
    minScaleUp: "100Gi"
    cooldown: "30m"
```

### Database Replica Set

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: postgres-replicas
  namespace: database
spec:
  selector:
    matchLabels:
      app: postgres
      role: replica
  coordinationPolicy: "largest"
  template:
    enabled: true
    threshold: 85.0
    increase: "20%"
    maxSize: "1000Gi"
    cooldown: "25m"
```

### Distributed Storage

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: ceph-osds
  namespace: storage
spec:
  selector:
    matchLabels:
      app: ceph
      component: osd
  coordinationPolicy: "sum"
  totalCapacity: "10Ti"  # Distribute 10Ti across all OSDs
  template:
    enabled: true
    threshold: 75.0
    increase: "10%"
    cooldown: "1h"
```

## Using PVCGroups

### 1. Create the PVCGroup

```bash
kubectl apply -f elasticsearch-group.yaml
```

### 2. Create Matching PVCs

The webhook automatically applies group settings:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: elasticsearch-data-0
  namespace: logging
  labels:
    app: elasticsearch      # Matches group selector
    component: data         # Matches group selector
    node: "0"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: elasticsearch-data-1
  namespace: logging
  labels:
    app: elasticsearch
    component: data
    node: "1"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 150Gi  # Different initial size
  storageClassName: fast-ssd
```

### 3. Verify Group Coordination

```bash
# Check group status
kubectl get pvcgroup elasticsearch-data -o yaml

# Check PVC sizes (should be coordinated)
kubectl get pvc -l app=elasticsearch,component=data
```

## Group Behavior

### Initial Coordination
When PVCs join a group with different sizes, they're coordinated according to the policy:

```
Before: PVC-A=100Gi, PVC-B=150Gi, PVC-C=120Gi
After (largest): All become 150Gi
```

### Expansion Coordination
When one PVC needs expansion, all group members expand together:

```
Trigger: PVC-A reaches 80% usage at 150Gi
Action: All PVCs expand to 180Gi (150Gi + 20%)
```

### New Member Addition
New PVCs automatically join the group and match existing sizes:

```
Existing: PVC-A=200Gi, PVC-B=200Gi
New: PVC-C=100Gi → automatically resized to 200Gi
```

## Template Fields

The template section defines expansion behavior for all group members:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `enabled` | bool | Enable auto-expansion | `true` |
| `threshold` | float64 | Storage usage threshold | `80.0` |
| `inodesThreshold` | float64 | Inode usage threshold | `85.0` |
| `increase` | string | Expansion amount | `"25%"` |
| `maxSize` | string | Maximum size per PVC | `"1000Gi"` |
| `minScaleUp` | string | Minimum expansion amount | `"50Gi"` |
| `cooldown` | string | Cooldown between expansions | `"30m"` |

## Advanced Configuration

### Conditional Groups

Use complex selectors for fine-grained control:

```yaml
spec:
  selector:
    matchLabels:
      app: database
    matchExpressions:
    - key: tier
      operator: In
      values: ["primary", "replica"]
    - key: environment
      operator: NotIn
      values: ["development"]
```

### Size Limits per Policy

Different policies can have different size constraints:

```yaml
# Largest policy with individual limits
spec:
  coordinationPolicy: "largest"
  template:
    maxSize: "500Gi"  # Each PVC max 500Gi

# Sum policy with total limit
spec:
  coordinationPolicy: "sum"
  totalCapacity: "2Ti"  # Total across all PVCs
```

## Monitoring Groups

### Group Status

```bash
# List all groups
kubectl get pvcgroup

# Get group details
kubectl describe pvcgroup elasticsearch-data

# Check group members
kubectl get pvc -l app=elasticsearch,component=data
```

### Group Metrics

Monitor group coordination with Prometheus:

```promql
# Group member count
pvcchonker_pvcgroup_members_total

# Group coordination events
rate(pvcchonker_pvcgroup_coordinations_total[5m])

# Group size distribution
pvcchonker_pvcgroup_size_bytes
```

## Troubleshooting

### Webhook Not Working

```bash
# Check webhook pod
kubectl get pods -n pvc-chonker-system

# Check webhook configuration
kubectl get validatingwebhookconfiguration
kubectl get mutatingwebhookconfiguration

# Test webhook
kubectl apply --dry-run=server -f test-pvc.yaml
```

### Group Not Coordinating

```bash
# Check if PVCs match selector
kubectl get pvc --show-labels

# Verify group exists
kubectl get pvcgroup -n your-namespace

# Check operator logs
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager
```

### Size Conflicts

If manual PVC resizing conflicts with group coordination, the operator will log warnings and attempt to reconcile on the next cycle.

## Best Practices

1. **Plan Group Strategy**: Choose coordination policy based on your use case
2. **Test Webhook**: Verify webhook is working before creating groups
3. **Monitor Coordination**: Watch for coordination events and conflicts
4. **Size Planning**: Consider total capacity when using sum policy
5. **Gradual Rollout**: Test groups with small clusters first
6. **Label Consistency**: Use consistent labeling for reliable selection

## Related Documentation

- **[PVCPolicy](./pvcpolicy.md)** - Individual PVC policies
- **[Configuration](./configuration.md)** - Complete configuration reference
- **[Examples](../examples/overview.md)** - Real-world group examples