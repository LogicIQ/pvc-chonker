# Annotation Reference

This document provides a complete reference for all PVC Chonker annotations.

## Core Annotations

### `pvc-chonker.io/enabled`
**Type**: `string` (boolean)  
**Default**: `"false"`  
**Description**: Enables or disables automatic expansion for this PVC.  
**Values**: `"true"` | `"false"`  
**Priority**: Highest - overrides all other settings  

```yaml
annotations:
  pvc-chonker.io/enabled: "true"   # Enable expansion
  pvc-chonker.io/enabled: "false"  # Disable expansion (break glass)
```

### `pvc-chonker.io/threshold`
**Type**: `string` (percentage)  
**Default**: `"80%"`  
**Description**: Storage usage percentage that triggers expansion.  
**Range**: `1%` to `99%`  
**Examples**: `"75%"`, `"85%"`, `"90%"`  

```yaml
annotations:
  pvc-chonker.io/threshold: "85%"  # Expand when 85% full
```

### `pvc-chonker.io/inodes-threshold`
**Type**: `string` (percentage)  
**Default**: `"80%"`  
**Description**: Inode usage percentage that triggers expansion.  
**Range**: `1%` to `99%`  
**Note**: Only effective on filesystems with dynamic inodes (XFS, Btrfs, ZFS)  

```yaml
annotations:
  pvc-chonker.io/inodes-threshold: "90%"  # Expand when 90% inodes used
```

### `pvc-chonker.io/increase`
**Type**: `string` (percentage or quantity)  
**Default**: `"10%"`  
**Description**: Amount to increase PVC size during expansion.  
**Formats**: 
- Percentage: `"20%"`, `"50%"`
- Quantity: `"5Gi"`, `"100Mi"`, `"1Ti"`

```yaml
annotations:
  pvc-chonker.io/increase: "25%"   # Increase by 25% of current size
  pvc-chonker.io/increase: "10Gi"  # Increase by exactly 10Gi
```

## Size Limits

### `pvc-chonker.io/max-size`
**Type**: `string` (quantity)  
**Default**: `none` (unlimited)  
**Description**: Maximum size the PVC can grow to.  
**Formats**: `"100Gi"`, `"1Ti"`, `"500Mi"`  

```yaml
annotations:
  pvc-chonker.io/max-size: "1000Gi"  # Never exceed 1000Gi
```

### `pvc-chonker.io/min-scale-up`
**Type**: `string` (quantity)  
**Default**: `"1Gi"`  
**Description**: Minimum amount to increase during expansion.  
**Purpose**: Ensures meaningful size increases, prevents tiny expansions  

```yaml
annotations:
  pvc-chonker.io/min-scale-up: "5Gi"  # Always increase by at least 5Gi
```

## Timing Controls

### `pvc-chonker.io/cooldown`
**Type**: `string` (duration)  
**Default**: `"15m"`  
**Description**: Minimum time between expansions for this PVC.  
**Formats**: `"30s"`, `"15m"`, `"2h"`, `"24h"`  
**Purpose**: Prevents rapid successive expansions  

```yaml
annotations:
  pvc-chonker.io/cooldown: "30m"  # Wait 30 minutes between expansions
```

## Metadata Annotations

### `pvc-chonker.io/group`
**Type**: `string`  
**Set by**: PVCGroup webhook (read-only)  
**Description**: Name of the PVCGroup this PVC belongs to.  
**Note**: Automatically set by webhook, do not set manually  

### `pvc-chonker.io/last-expansion`
**Type**: `string` (RFC3339 timestamp)  
**Set by**: Controller (read-only)  
**Description**: Timestamp of the last successful expansion.  
**Example**: `"2024-01-15T10:30:00Z"`  

### `pvc-chonker.io/disabled-reason`
**Type**: `string`  
**Optional**: User-defined  
**Description**: Human-readable reason why expansion is disabled.  
**Use with**: `pvc-chonker.io/enabled: "false"`  

```yaml
annotations:
  pvc-chonker.io/enabled: "false"
  pvc-chonker.io/disabled-reason: "maintenance-window"
```

## Complete Example

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-storage
  annotations:
    # Core settings
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "85%"
    pvc-chonker.io/inodes-threshold: "90%"
    pvc-chonker.io/increase: "25%"
    
    # Size limits
    pvc-chonker.io/max-size: "2000Gi"
    pvc-chonker.io/min-scale-up: "10Gi"
    
    # Timing
    pvc-chonker.io/cooldown: "30m"
    
    # Optional metadata
    pvc-chonker.io/disabled-reason: ""  # Empty when enabled
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

## Validation Rules

### Threshold Validation
- Must be between 1% and 99%
- Storage and inode thresholds are independent
- Expansion triggers when EITHER threshold is reached

### Increase Validation
- Percentage increases are calculated from current PVC size
- Quantity increases are added to current size
- Final size is rounded up to next GiB boundary
- Must respect `min-scale-up` setting

### Size Validation
- New size cannot exceed `max-size` if set
- New size must be larger than current size
- Storage class must support `allowVolumeExpansion: true`

## Priority and Inheritance

Annotations have the highest priority in the configuration hierarchy:

1. **PVC Annotations** ← Highest priority
2. PVCGroup Template
3. PVCPolicy Template  
4. Global Configuration
5. Built-in Defaults

**Example Priority Resolution:**
```yaml
# If PVC has: threshold="90%"
# And PVCGroup has: threshold="80%", increase="25%"
# And PVCPolicy has: increase="20%", cooldown="20m"
# And Global has: cooldown="15m"

# Final effective configuration:
# threshold: "90%" (from PVC annotation)
# increase: "25%" (from PVCGroup - highest available)
# cooldown: "20m" (from PVCPolicy - highest available)
```

## Common Patterns

### Conservative Database
```yaml
annotations:
  pvc-chonker.io/enabled: "true"
  pvc-chonker.io/threshold: "90%"      # Wait until very full
  pvc-chonker.io/increase: "50%"       # Large increases
  pvc-chonker.io/cooldown: "1h"        # Long cooldown
  pvc-chonker.io/max-size: "5000Gi"    # Generous limit
```

### Aggressive Log Storage
```yaml
annotations:
  pvc-chonker.io/enabled: "true"
  pvc-chonker.io/threshold: "75%"      # Expand early
  pvc-chonker.io/increase: "100%"      # Double the size
  pvc-chonker.io/cooldown: "5m"        # Short cooldown
  pvc-chonker.io/max-size: "1000Gi"    # Reasonable limit
```

### Maintenance Mode
```yaml
annotations:
  pvc-chonker.io/enabled: "false"
  pvc-chonker.io/disabled-reason: "scheduled-maintenance"
```

## Troubleshooting

### Annotation Not Working
1. Check annotation spelling and format
2. Verify PVC has correct storage class
3. Check controller logs for validation errors
4. Ensure no conflicting policies or groups

### Invalid Values
```bash
# Check PVC events for validation errors
kubectl describe pvc your-pvc

# Check controller logs
kubectl logs -n pvc-chonker-system deployment/controller-manager
```

### Priority Conflicts
Use `kubectl describe pvc` to see which configuration source is being used:
```yaml
# Controller adds status annotations showing effective config
metadata:
  annotations:
    pvc-chonker.io/effective-threshold: "85%"
    pvc-chonker.io/effective-source: "annotation"  # or "group", "policy", "global"
```