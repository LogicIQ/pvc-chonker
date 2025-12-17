# Advanced Features: PVCPolicy and PVCGroup

> ⚠️ **PLANNED FEATURES** - These advanced features are not yet implemented. This documentation describes the planned functionality for future releases. Currently, only annotation-based configuration is supported.

## Overview

PVC Chonker will support advanced configuration management through Custom Resource Definitions (CRDs) that complement the existing annotation-based approach. These features will provide enterprise-grade policy management and group coordination capabilities.

## Configuration Hierarchy

The configuration precedence order is:
1. **PVC Annotations** (highest priority - per-PVC overrides)
2. **PVCGroup CRD** (group coordination settings)
3. **PVCPolicy CRD** (policy templates)
4. **Global Flags/Environment Variables**
5. **Built-in Defaults** (fallback)

### Override Annotation
Use `pvc-chonker.io/enabled: "false"` to completely disable expansion for a PVC, regardless of any policy or group configuration. This annotation takes highest priority and will prevent any expansion from occurring.

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

### Advanced Selector Example
```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: high-performance-policy
spec:
  selector:
    matchLabels:
      tier: production
      performance: high
    matchExpressions:
    - key: app
      operator: In
      values: ["elasticsearch", "mongodb", "postgresql"]
  template:
    threshold: "75%"
    increase: "50%"
    maxSize: "5000Gi"
    cooldown: "15m"
```

### PVC Integration
PVCs automatically inherit policy settings when labels match, but annotations always take precedence:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  labels:
    workload: database  # Matches database-policy selector
    tier: production
  annotations:
    # Individual annotations override policy settings
    pvc-chonker.io/threshold: "90%"  # Overrides policy's 85%
    pvc-chonker.io/enabled: "false"    # Disables expansion entirely
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

## PVCGroup Controller

### Purpose
Coordinate PVC expansion across related volumes to maintain consistent sizing within a group. Groups operate independently of policies - a PVC can be in a group without having a policy applied.

### When to Use
- **Clustered applications** (Elasticsearch, MongoDB replica sets)
- **Distributed storage** requiring uniform capacity
- **Load balancing** across multiple volumes
- **Consistent sizing** for related workloads

### Important Notes
- **PVCPolicy settings are NOT applied to PVCGroups** - groups have their own template settings
- **Individual PVC annotations always override group settings**
- **Groups coordinate sizing, policies provide templates**

### Basic Example
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
    threshold: "80%"
    increase: "20%"
    maxSize: "1000Gi"
    cooldown: "20m"
```

### Coordination Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `largest` | All PVCs match the largest size in group | Default, ensures no volume is smaller |
| `average` | All PVCs match the average size in group | Balanced approach for cost optimization |
| `sum` | Distribute total capacity evenly across PVCs | Fixed total capacity scenarios |

### Advanced Group Example
```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: mongodb-replica-set
spec:
  selector:
    matchLabels:
      app: mongodb
      role: replica
  coordinationPolicy: "largest"
  template:
    # Group-specific settings (independent of any PVCPolicy)
    threshold: "85%"
    inodesThreshold: "90%"
    increase: "30%"
    maxSize: "2000Gi"
    minScaleUp: "100Gi"
    cooldown: "45m"
  status:
    currentSize: "500Gi"
    lastExpansion: "2024-01-15T10:30:00Z"
    memberCount: 3
```

### PVC with Group Override
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mongodb-data-1
  labels:
    app: mongodb
    role: replica  # Matches group selector
  annotations:
    # This PVC gets different threshold despite being in group
    pvc-chonker.io/threshold: "95%"  # Overrides group's 85%
    pvc-chonker.io/enabled: "false"    # Skips expansion entirely
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 500Gi
```

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