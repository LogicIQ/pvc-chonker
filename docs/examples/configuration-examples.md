# Configuration Examples

This document provides real-world examples of PVC Chonker configurations for different use cases.

## Database Storage Examples

### PostgreSQL Primary Database
High-performance database requiring conservative expansion:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-primary-data
  labels:
    app: postgresql
    role: primary
    tier: database
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "85%"           # Conservative threshold
    pvc-chonker.io/inodes-threshold: "90%"    # Monitor inodes closely
    pvc-chonker.io/increase: "50%"            # Significant increases
    pvc-chonker.io/max-size: "5000Gi"         # Large limit for growth
    pvc-chonker.io/min-scale-up: "20Gi"       # Meaningful minimum increase
    pvc-chonker.io/cooldown: "1h"             # Conservative cooldown
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

### MongoDB Replica Set (with PVCGroup)
Coordinated expansion across replica set members:

```yaml
# PVCGroup for coordination
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: mongodb-replica-set
  namespace: database
spec:
  selector:
    matchLabels:
      app: mongodb
      cluster: main
  coordinationPolicy: "largest"  # All replicas match largest member
  template:
    enabled: true
    threshold: "80%"
    increase: "30%"
    maxSize: "2000Gi"
    cooldown: "45m"
---
# Individual replica PVCs
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mongodb-replica-0
  labels:
    app: mongodb
    cluster: main
    replica: "0"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 200Gi
  storageClassName: fast-ssd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mongodb-replica-1
  labels:
    app: mongodb
    cluster: main
    replica: "1"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 200Gi
  storageClassName: fast-ssd
```

## Application Storage Examples

### Web Application Uploads
Fast-growing user upload storage:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: webapp-uploads
  labels:
    app: webapp
    component: storage
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "70%"           # Expand early
    pvc-chonker.io/increase: "100%"           # Double the size
    pvc-chonker.io/max-size: "10000Gi"        # Large limit
    pvc-chonker.io/min-scale-up: "5Gi"        # Reasonable minimum
    pvc-chonker.io/cooldown: "10m"            # Short cooldown
spec:
  accessModes: [ReadWriteMany]
  resources:
    requests:
      storage: 50Gi
  storageClassName: standard
```

### Application Cache
Temporary cache storage with aggressive expansion:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-cache
  labels:
    app: redis
    role: cache
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "75%"
    pvc-chonker.io/increase: "200%"           # Triple the size
    pvc-chonker.io/max-size: "500Gi"          # Reasonable cache limit
    pvc-chonker.io/cooldown: "5m"             # Very short cooldown
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
  storageClassName: fast-ssd
```

## Log Storage Examples

### Application Logs
High-volume log storage with predictable growth:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-logs
  labels:
    app: myapp
    component: logs
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
    pvc-chonker.io/increase: "50%"
    pvc-chonker.io/max-size: "1000Gi"
    pvc-chonker.io/cooldown: "15m"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 20Gi
  storageClassName: standard
```

### Elasticsearch Data Nodes
Coordinated log storage cluster:

```yaml
# PVCGroup for Elasticsearch cluster
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-data
  namespace: logging
spec:
  selector:
    matchLabels:
      app: elasticsearch
      role: data
  coordinationPolicy: "average"  # Balanced sizing
  template:
    enabled: true
    threshold: "75%"
    increase: "25%"
    maxSize: "3000Gi"
    cooldown: "30m"
---
# Data node PVCs (created by StatefulSet)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: elasticsearch-data-0
  labels:
    app: elasticsearch
    role: data
    node: "0"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 500Gi
  storageClassName: fast-ssd
```

## Policy-Based Examples

### Multi-Tenant Database Policy
Centralized policy for tenant databases:

```yaml
# Policy for all tenant databases
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: tenant-database-policy
  namespace: tenants
spec:
  selector:
    matchLabels:
      workload: database
      tier: tenant
  template:
    enabled: true
    threshold: 80.0
    inodesThreshold: 85.0
    increase: "25%"
    maxSize: "1000Gi"
    minScaleUp: "10Gi"
    cooldown: "30m"
---
# Tenant PVCs automatically inherit policy
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tenant-a-db
  labels:
    workload: database
    tier: tenant
    tenant: "a"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: standard
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tenant-b-db
  labels:
    workload: database
    tier: tenant
    tenant: "b"
  annotations:
    # Override policy for special tenant
    pvc-chonker.io/max-size: "2000Gi"  # Larger limit
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

### Development vs Production Policies
Different policies for different environments:

```yaml
# Production policy - conservative
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: production-policy
  namespace: production
spec:
  selector:
    matchLabels:
      environment: production
  template:
    enabled: true
    threshold: 85.0
    increase: "25%"
    maxSize: "5000Gi"
    cooldown: "1h"
---
# Development policy - aggressive
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: development-policy
  namespace: development
spec:
  selector:
    matchLabels:
      environment: development
  template:
    enabled: true
    threshold: 70.0
    increase: "100%"
    maxSize: "500Gi"
    cooldown: "5m"
```

## Special Use Cases

### Backup Storage
Large, infrequent expansions for backup data:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: backup-storage
  labels:
    app: backup
    type: archive
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "90%"           # Wait until very full
    pvc-chonker.io/increase: "500Gi"          # Fixed large increase
    pvc-chonker.io/max-size: "50000Gi"        # Very large limit
    pvc-chonker.io/cooldown: "24h"            # Daily expansion at most
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1000Gi
  storageClassName: cold-storage
```

### Temporary Processing Storage
Short-lived storage with aggressive expansion:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: batch-processing-temp
  labels:
    app: batch-processor
    type: temporary
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "60%"           # Very early expansion
    pvc-chonker.io/increase: "300%"           # Quadruple the size
    pvc-chonker.io/max-size: "2000Gi"         # Reasonable limit
    pvc-chonker.io/cooldown: "1m"             # Almost no cooldown
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
  storageClassName: fast-ssd
```

### Maintenance Mode Examples
Temporarily disabling expansion:

```yaml
# Disable during maintenance window
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-maintenance
  annotations:
    pvc-chonker.io/enabled: "false"
    pvc-chonker.io/disabled-reason: "maintenance-window-2024-01-15"
    # Previous settings preserved for re-enabling
    pvc-chonker.io/threshold: "85%"
    pvc-chonker.io/increase: "25%"
spec:
  # ... PVC spec
```

```yaml
# Permanently exclude from auto-expansion
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: manual-managed-storage
  labels:
    app: special-app
    management: manual
  annotations:
    pvc-chonker.io/enabled: "false"
    pvc-chonker.io/disabled-reason: "requires-manual-capacity-planning"
spec:
  # ... PVC spec
```

## Complex Coordination Example

### Multi-Tier Application with Mixed Policies
Application with different storage tiers and coordination:

```yaml
# Fast tier group (SSDs)
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: app-fast-tier
  namespace: myapp
spec:
  selector:
    matchLabels:
      app: myapp
      tier: fast
  coordinationPolicy: "largest"
  template:
    enabled: true
    threshold: "80%"
    increase: "30%"
    maxSize: "1000Gi"
    cooldown: "20m"
---
# Standard tier policy
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: app-standard-policy
  namespace: myapp
spec:
  selector:
    matchLabels:
      app: myapp
      tier: standard
  template:
    enabled: true
    threshold: 75.0
    increase: "50%"
    maxSize: "5000Gi"
    cooldown: "1h"
---
# Fast tier PVCs (coordinated)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-cache-1
  labels:
    app: myapp
    tier: fast
    component: cache
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
  name: app-cache-2
  labels:
    app: myapp
    tier: fast
    component: cache
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
---
# Standard tier PVCs (policy-based)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data-1
  labels:
    app: myapp
    tier: standard
    component: data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 500Gi
  storageClassName: standard
---
# Override for special data volume
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data-critical
  labels:
    app: myapp
    tier: standard
    component: data
  annotations:
    # Override policy settings
    pvc-chonker.io/threshold: "90%"    # More conservative
    pvc-chonker.io/max-size: "10000Gi" # Higher limit
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1000Gi
  storageClassName: standard
```

## Migration Examples

### Migrating from Manual to Automatic
Gradually enabling auto-expansion:

```yaml
# Phase 1: Enable with conservative settings
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: legacy-database
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "95%"     # Very conservative
    pvc-chonker.io/increase: "10%"      # Small increases
    pvc-chonker.io/cooldown: "24h"      # Long cooldown
    pvc-chonker.io/max-size: "2000Gi"   # Safety limit
spec:
  # ... existing PVC spec
```

```yaml
# Phase 2: Optimize settings based on experience
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: legacy-database
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "85%"     # More aggressive
    pvc-chonker.io/increase: "25%"      # Larger increases
    pvc-chonker.io/cooldown: "2h"       # Shorter cooldown
    pvc-chonker.io/max-size: "5000Gi"   # Higher limit
spec:
  # ... existing PVC spec
```