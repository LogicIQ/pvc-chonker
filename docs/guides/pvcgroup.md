# PVCGroup Configuration

PVCGroup enables coordinated expansion across multiple related PVCs, ensuring all volumes in a group maintain the same size.

## Overview

PVCGroup provides:
- **Coordinated Expansion**: Keep related PVCs at the same size (largest in group)
- **Annotation-based Membership**: Explicit PVC enrollment via annotations
- **Template Management**: Centralized configuration for group members
- **Override Support**: Individual PVC annotations take precedence

## Prerequisites

PVCGroups work with or without the webhook, but the webhook provides automatic template application for new PVCs.

## Basic PVCGroup

```yaml
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

## Coordination Behavior

PVCGroups use a **largest size policy** - all PVCs in the group match the size of the largest PVC:

**Example**: If PVCs are 100Gi, 150Gi, 120Gi → all become 150Gi

This ensures:
- No volume ever gets smaller
- Consistent sizing across related volumes
- Safe, predictable behavior

## Configuration Examples

### Elasticsearch Cluster

```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-data
  namespace: logging
spec:
  template:
    threshold: "80%"
    inodesThreshold: "85%"
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
  template:
    threshold: "85%"
    increase: "20%"
    maxSize: "1000Gi"
    cooldown: "25m"
```

## Using PVCGroups

### 1. Create the PVCGroup

```bash
kubectl apply -f elasticsearch-group.yaml
```

### 2. Annotate PVCs to Join the Group

PVCs must have both annotations to be managed by the group:

```bash
# Add PVCs to the group
kubectl annotate pvc elasticsearch-data-0 \
  pvc-chonker.io/group=elasticsearch-data \
  pvc-chonker.io/enabled=true

kubectl annotate pvc elasticsearch-data-1 \
  pvc-chonker.io/group=elasticsearch-data \
  pvc-chonker.io/enabled=true

# Or annotate multiple PVCs at once
kubectl annotate pvc -l app=elasticsearch \
  pvc-chonker.io/group=elasticsearch-data \
  pvc-chonker.io/enabled=true
```

### 3. Verify Group Coordination

```bash
# Check group status
kubectl get pvcgroup elasticsearch-data -o yaml

# Check PVC sizes (should be coordinated)
kubectl get pvc -l app=elasticsearch
```

## Group Behavior

### Initial Coordination
When PVCs join a group with different sizes, they're coordinated to the largest size:

```
Before: PVC-A=100Gi, PVC-B=150Gi, PVC-C=120Gi
After: All become 150Gi (largest size)
```

### Expansion Coordination
When one PVC needs expansion, all group members expand together:

```
Trigger: PVC-A reaches 80% usage at 150Gi
Action: All PVCs expand to 180Gi (150Gi + 20%)
```

### New Member Addition
New PVCs with group annotations automatically match existing sizes:

```
Existing: PVC-A=200Gi, PVC-B=200Gi
New: PVC-C=100Gi → automatically resized to 200Gi
```

## Required Annotations

PVCs must have both annotations to be managed by a PVCGroup:

| Annotation | Value | Description |
|------------|-------|-------------|
| `pvc-chonker.io/group` | `<group-name>` | Which group this PVC belongs to |
| `pvc-chonker.io/enabled` | `"true"` | Enable auto-expansion for this PVC |

Example PVC:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: elasticsearch-data-0
  namespace: logging
  annotations:
    pvc-chonker.io/group: elasticsearch-data
    pvc-chonker.io/enabled: "true"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

## Template Fields

The template section defines expansion behavior for all group members:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `threshold` | string | Storage usage threshold | `"80%"` |
| `inodesThreshold` | string | Inode usage threshold | `"85%"` |
| `increase` | string | Expansion amount | `"25%"` |
| `maxSize` | string | Maximum size per PVC | `"1000Gi"` |
| `minScaleUp` | string | Minimum expansion amount | `"50Gi"` |
| `cooldown` | string | Cooldown between expansions | `"30m"` |

## Monitoring Groups

### Group Status

```bash
# List all groups
kubectl get pvcgroup

# Get group details
kubectl describe pvcgroup elasticsearch-data

# Check group members
kubectl get pvc -o custom-columns="NAME:.metadata.name,GROUP:.metadata.annotations.pvc-chonker\.io/group,ENABLED:.metadata.annotations.pvc-chonker\.io/enabled,SIZE:.spec.resources.requests.storage"
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

### Group Not Coordinating

```bash
# Check if PVCs have required annotations
kubectl get pvc -o yaml | grep -A5 -B5 "pvc-chonker.io"

# Verify group exists
kubectl get pvcgroup -n your-namespace

# Check operator logs
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager
```

### PVC Not Joining Group

Ensure PVC has both required annotations:
```bash
kubectl annotate pvc my-pvc \
  pvc-chonker.io/group=my-group \
  pvc-chonker.io/enabled=true
```

### Size Conflicts

If manual PVC resizing conflicts with group coordination, the operator will log warnings and attempt to reconcile on the next cycle.

## Best Practices

1. **Explicit Membership**: Always use both required annotations for clear group membership
2. **Consistent Naming**: Use descriptive group names that match your application architecture
3. **Monitor Coordination**: Watch for coordination events and conflicts
4. **Test First**: Test groups with small clusters before production deployment
5. **Override When Needed**: Use individual PVC annotations to override group settings when necessary

## Migration from Label Selectors

If migrating from an older version that used label selectors, update your workflow:

**Old approach (deprecated):**
```yaml
spec:
  selector:
    matchLabels:
      app: elasticsearch
```

**New approach:**
```bash
# Manually annotate PVCs
kubectl annotate pvc -l app=elasticsearch \
  pvc-chonker.io/group=elasticsearch-data \
  pvc-chonker.io/enabled=true
```

## Related Documentation

- **[PVCPolicy](./pvcpolicy.md)** - Individual PVC policies
- **[Configuration](./configuration.md)** - Complete configuration reference
- **[Examples](../examples/overview.md)** - Real-world group examples