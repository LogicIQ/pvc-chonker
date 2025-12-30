# Advanced Features

> ⚠️ **CURRENT IMPLEMENTATION** - PVCGroup uses annotation-based membership with largest size coordination policy only.

## Overview

PVC Chonker supports advanced configuration management through Custom Resource Definitions (CRDs) that complement the existing annotation-based approach.

## Configuration Hierarchy

The configuration precedence order is:
1. **PVC Annotations** (highest priority - per-PVC overrides)
2. **PVCGroup CRD** (group coordination settings)
3. **PVCPolicy CRD** (policy templates)
4. **Global Flags/Environment Variables**
5. **Built-in Defaults** (fallback)

### Override Annotation
Use `pvc-chonker.io/enabled: "false"` to completely disable expansion for a PVC, regardless of any policy or group configuration.

## PVCPolicy Controller

### Purpose
Manage reusable expansion policies through Kubernetes CRDs instead of repeating annotations on individual PVCs.

### When to Use
- **Multiple PVCs** with similar expansion requirements
- **Standardized policies** across teams or applications
- **Centralized management** of expansion settings
- **Policy inheritance** for new PVCs

### Basic Example
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
    threshold: "85%"
    inodesThreshold: "90%"
    increase: "25%"
    maxSize: "2000Gi"
    minScaleUp: "50Gi"
    cooldown: "30m"
```

## PVCGroup Controller

### Purpose
Coordinate PVC expansion across related volumes to maintain consistent sizing within a group using the largest size policy.

### When to Use
- **Clustered applications** (Elasticsearch, MongoDB replica sets)
- **Distributed storage** requiring uniform capacity
- **Consistent sizing** for related workloads

### Basic Example
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

### PVC Membership
PVCs join groups via annotations:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: elasticsearch-data-0
  annotations:
    pvc-chonker.io/group: elasticsearch-cluster
    pvc-chonker.io/enabled: "true"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
```

### Coordination Behavior

PVCGroups use **largest size policy** - all PVCs in the group match the size of the largest PVC:

**Example**: If PVCs are 100Gi, 150Gi, 120Gi → all become 150Gi

This ensures:
- No volume ever gets smaller
- Consistent sizing across related volumes
- Safe, predictable behavior

## Annotations vs CRDs Comparison

### Annotations Approach
```yaml
# Per-PVC configuration (current method)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
    pvc-chonker.io/increase: "20%"
    pvc-chonker.io/max-size: "500Gi"
    pvc-chonker.io/cooldown: "15m"
    pvc-chonker.io/enabled: "false"  # Break glass - disables all expansion
```

**Pros:**
- Simple and direct
- No additional CRDs required
- Works immediately
- Per-PVC customization

**Cons:**
- Repetitive for similar PVCs
- Hard to manage at scale
- No centralized policy management
- Manual updates required

### CRDs Approach
```yaml
# Policy-based configuration (advanced method)
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: standard-app-policy
spec:
  selector:
    matchLabels:
      tier: application
  template:
    enabled: true
    threshold: "80%"
    increase: "20%"
    maxSize: "500Gi"
    cooldown: "15m"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  labels:
    tier: application  # Inherits from policy
```

**Pros:**
- Centralized management
- Reusable policies
- Easier to maintain at scale
- Consistent configuration
- Group coordination capabilities

**Cons:**
- Additional complexity
- Requires CRD installation
- Learning curve for teams

## Migration Strategy

### Phase 1: Gradual Adoption
1. Keep existing annotation-based PVCs unchanged
2. Create policies for new PVCs
3. Test policy inheritance

### Phase 2: Policy Migration
1. Create policies matching existing annotation patterns
2. Add labels to existing PVCs
3. Remove redundant annotations

### Phase 3: Group Coordination
1. Identify PVCs that should be grouped
2. Create PVCGroup resources
3. Monitor coordination behavior

## Best Practices

### Policy Design
- **Use descriptive names** for policies and groups
- **Start with conservative settings** and adjust based on monitoring
- **Group related policies** in the same namespace
- **Document policy purposes** in metadata annotations

### Label Strategy
- **Consistent labeling** across related resources
- **Hierarchical labels** (app, component, tier)
- **Avoid label conflicts** between different systems

### Monitoring
- **Track policy application** through controller logs
- **Monitor group coordination** events
- **Set up alerts** for expansion failures
- **Review policies** regularly based on usage patterns

## Troubleshooting

### Policy Not Applied
1. Check label selectors match PVC labels
2. Verify policy is in correct namespace
3. Review controller logs for errors

### Group Coordination Issues
1. Confirm all PVCs have matching labels
2. Check coordination policy is valid
3. Monitor PVCGroup status for errors

### Configuration Conflicts
1. Review configuration hierarchy (Annotations > Group > Policy > Global > Defaults)
2. Check for conflicting policies and groups
3. Use `pvc-chonker.io/enabled: "false"` annotation to disable expansion
4. Use dry-run mode to test changes

### Disable Expansion Examples
```yaml
# Temporarily disable expansion during maintenance
metadata:
  annotations:
    pvc-chonker.io/enabled: "false"
    pvc-chonker.io/disabled-reason: "maintenance-window"

# Permanently exclude from expansion (overrides any policy/group)
metadata:
  labels:
    app: database  # Would normally match a policy
  annotations:
    pvc-chonker.io/enabled: "false"  # Takes precedence over policy
    pvc-chonker.io/disabled-reason: "manual-management"

# PVC in group but individually disabled
metadata:
  labels:
    app: elasticsearch
    cluster: main  # Matches group selector
  annotations:
    pvc-chonker.io/enabled: "false"  # Excluded from group coordination
```