# Prometheus Metrics

PVC Chonker exports comprehensive Prometheus metrics for monitoring and alerting. All metrics use the `pvcchonker` namespace with appropriate subsystems.

## Resizer Metrics

### Success/Failure Counters
- `pvcchonker_resizer_success_resize_total{persistentvolumeclaim, namespace}` - Successful PVC expansions
- `pvcchonker_resizer_failed_resize_total{persistentvolumeclaim, namespace, reason}` - Failed PVC expansions with reason
- `pvcchonker_resizer_threshold_reached_total{persistentvolumeclaim, namespace}` - Times threshold was reached
- `pvcchonker_resizer_limit_reached_total{persistentvolumeclaim, namespace}` - Times max size limit was reached

### Operational Counters
- `pvcchonker_resizer_cooldown_skipped_total{persistentvolumeclaim, namespace}` - PVCs skipped due to cooldown
- `pvcchonker_resizer_resize_in_progress_total{persistentvolumeclaim, namespace}` - PVCs skipped due to ongoing resize
- `pvcchonker_resizer_loop_seconds_total` - Total seconds spent in reconciliation loops

## Client Metrics

### Kubernetes API Client
- `pvcchonker_kubernetes_client_requests_total{operation, status}` - Total API requests by operation and status
- `pvcchonker_kubernetes_client_fail_total{operation}` - Failed API requests by operation

### Kubelet Client
- `pvcchonker_kubelet_client_requests_total{status}` - Total kubelet requests by status
- `pvcchonker_kubelet_client_fail_total` - Failed kubelet requests
- `pvcchonker_kubelet_client_response_time_seconds` - Kubelet response time histogram

## Operational Metrics

### System Status
- `pvcchonker_last_reconciliation_timestamp_seconds` - Timestamp of last reconciliation
- `pvcchonker_reconciliation_status{status}` - Last reconciliation status (success/failure)
- `pvcchonker_managed_pvcs_total` - Total number of managed PVCs

### PVC Status
- `pvcchonker_pvc_usage_percent{persistentvolumeclaim, namespace}` - Current PVC storage usage percentage
- `pvcchonker_pvc_capacity_bytes{persistentvolumeclaim, namespace}` - Current PVC capacity in bytes
- `pvcchonker_pvc_inodes_usage_percent{persistentvolumeclaim, namespace}` - Current PVC inode usage percentage
- `pvcchonker_pvc_inodes_total{persistentvolumeclaim, namespace}` - Total inodes available in PVC

> **Note**: Inode metrics are only available for volumes that expose inode statistics via kubelet. ext3/ext4 filesystems have fixed inode counts that don't increase with volume expansion.

## Failure Reasons

The `reason` label in `pvcchonker_resizer_failed_resize_total` includes:
- `storage_class_not_expandable` - Storage class doesn't allow expansion
- `metrics_not_found` - Volume metrics unavailable
- `expansion_failed` - PVC update operation failed

## Example Queries

### Alert on High Failure Rate
```promql
rate(pvcchonker_resizer_failed_resize_total[5m]) > 0.1
```

### Monitor PVC Usage
```promql
# Storage usage
pvcchonker_pvc_usage_percent > 80

# Inode usage (filesystem-dependent)
pvcchonker_pvc_inodes_usage_percent > 80

# Alert on inode pressure for ext3/ext4 (expansion won't help)
pvcchonker_pvc_inodes_usage_percent > 90 and pvcchonker_pvc_inodes_total > 0
```

### Track Expansion Success Rate
```promql
rate(pvcchonker_resizer_success_resize_total[5m]) / 
(rate(pvcchonker_resizer_success_resize_total[5m]) + rate(pvcchonker_resizer_failed_resize_total[5m]))
```

### Monitor Kubelet Connectivity
```promql
rate(pvcchonker_kubelet_client_fail_total[5m]) > 0
```

## Comparison with TopoLVM PVC Autoresizer

Our metrics provide equivalent or superior coverage compared to TopoLVM:

| TopoLVM Metric | PVC Chonker Equivalent | Enhancement |
|---|---|---|
| `pvcautoresizer_success_resize_total` | `pvcchonker_resizer_success_resize_total` | Same functionality |
| `pvcautoresizer_failed_resize_total` | `pvcchonker_resizer_failed_resize_total` | Added failure reasons |
| `pvcautoresizer_loop_seconds_total` | `pvcchonker_resizer_loop_seconds_total` | Same functionality |
| `pvcautoresizer_limit_reached_total` | `pvcchonker_resizer_limit_reached_total` | Same functionality |
| `pvcautoresizer_kubernetes_client_fail_total` | `pvcchonker_kubernetes_client_fail_total` | Added operation labels |
| `pvcautoresizer_metrics_client_fail_total` | `pvcchonker_kubelet_client_fail_total` | Renamed for clarity |
| - | `pvcchonker_resizer_threshold_reached_total` | **New**: Track threshold events |
| - | `pvcchonker_resizer_cooldown_skipped_total` | **New**: Track cooldown behavior |
| - | `pvcchonker_resizer_resize_in_progress_total` | **New**: Track concurrent resizes |
| - | `pvcchonker_pvc_usage_percent` | **New**: Real-time storage usage monitoring |
| - | `pvcchonker_pvc_capacity_bytes` | **New**: Capacity tracking |
| - | `pvcchonker_pvc_inodes_usage_percent` | **New**: Real-time inode usage monitoring |
| - | `pvcchonker_pvc_inodes_total` | **New**: Inode capacity tracking |
| - | `pvcchonker_kubelet_client_response_time_seconds` | **New**: Performance monitoring |

## Grafana Dashboard

A sample Grafana dashboard configuration is available in `examples/grafana-dashboard.json` with panels for:
- Expansion success/failure rates
- PVC usage trends
- System health indicators
- Client performance metrics