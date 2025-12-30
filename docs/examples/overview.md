# Examples

Real-world examples demonstrating how to use PVC Chonker for automatic storage expansion in different scenarios.

## Example Categories

### Basic Usage
- **[Database Storage](./database-storage.md)** - Auto-expanding database volumes
- **[Log Storage](./log-storage.md)** - Managing log volume growth
- **[Application Data](./application-data.md)** - General application storage

### Advanced Configuration
- **[Multi-Tenant](./multi-tenant.md)** - Policy-based tenant storage
- **[Coordinated Expansion](./coordinated-expansion.md)** - PVCGroup examples
- **[Policy Management](./policy-management.md)** - PVCPolicy examples

### Cloud Providers
- **[AWS EKS](./aws-eks.md)** - EBS volume expansion
- **[Google GKE](./google-gke.md)** - Persistent disk expansion
- **[Azure AKS](./azure-aks.md)** - Azure disk expansion

## Quick Reference

### Basic Annotation Pattern
```yaml
metadata:
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
    pvc-chonker.io/increase: "20%"
```

### Policy-Based Pattern
```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCPolicy
metadata:
  name: database-policy
spec:
  selector:
    matchLabels:
      workload: database
  template:
    enabled: true
    threshold: 85.0
    increase: "25%"
```

### Group Coordination Pattern
```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: cluster-storage
spec:
  template:
    threshold: "80%"
    increase: "20%"
    maxSize: "1000Gi"
```

PVCs join via annotations:
```bash
kubectl annotate pvc my-pvc \
  pvc-chonker.io/group=cluster-storage \
  pvc-chonker.io/enabled=true
```

## Common Scenarios

### High-Growth Applications
For applications with unpredictable storage growth:

```yaml
annotations:
  pvc-chonker.io/enabled: "true"
  pvc-chonker.io/threshold: "70%"    # Earlier expansion
  pvc-chonker.io/increase: "50%"     # Larger increases
  pvc-chonker.io/cooldown: "10m"     # Shorter cooldown
```

### Cost-Conscious Environments
For environments where storage costs matter:

```yaml
annotations:
  pvc-chonker.io/enabled: "true"
  pvc-chonker.io/threshold: "90%"    # Later expansion
  pvc-chonker.io/increase: "10%"     # Smaller increases
  pvc-chonker.io/max-size: "100Gi"   # Size limits
```

### Critical Production Systems
For systems that cannot tolerate storage outages:

```yaml
annotations:
  pvc-chonker.io/enabled: "true"
  pvc-chonker.io/threshold: "75%"    # Early expansion
  pvc-chonker.io/increase: "25%"     # Adequate headroom
  pvc-chonker.io/min-scale-up: "10Gi" # Meaningful increases
```

## Integration Examples

### Helm Charts
Integrate PVC Chonker annotations into your Helm charts:

```yaml
# templates/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "app.fullname" . }}-data
  annotations:
    pvc-chonker.io/enabled: {{ .Values.storage.autoExpand.enabled | quote }}
    pvc-chonker.io/threshold: {{ .Values.storage.autoExpand.threshold | quote }}
    pvc-chonker.io/increase: {{ .Values.storage.autoExpand.increase | quote }}
spec:
  accessModes: {{ .Values.storage.accessModes }}
  resources:
    requests:
      storage: {{ .Values.storage.size }}
```

### Kustomize
Use Kustomize to add annotations:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- base-pvc.yaml

patchesStrategicMerge:
- pvc-annotations.yaml
```

```yaml
# pvc-annotations.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
```

### Terraform
Manage PVC Chonker resources with Terraform:

```hcl
resource "kubernetes_persistent_volume_claim" "app_data" {
  metadata {
    name = "app-data"
    annotations = {
      "pvc-chonker.io/enabled"   = "true"
      "pvc-chonker.io/threshold" = "80%"
      "pvc-chonker.io/increase"  = "20%"
    }
  }
  
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "10Gi"
      }
    }
    storage_class_name = var.storage_class
  }
}
```

## Monitoring Examples

### Prometheus Alerts
Set up alerts for PVC expansion events:

```yaml
groups:
- name: pvc-chonker
  rules:
  - alert: PVCExpansionFailed
    expr: increase(pvcchonker_resizer_failed_resize_total[5m]) > 0
    labels:
      severity: warning
    annotations:
      summary: "PVC expansion failed"
      description: "PVC {{ $labels.pvc_name }} failed to expand"

  - alert: PVCNearMaxSize
    expr: pvcchonker_pvc_size_bytes / pvcchonker_pvc_max_size_bytes > 0.9
    labels:
      severity: warning
    annotations:
      summary: "PVC approaching maximum size"
```

### Grafana Dashboard
Monitor PVC usage and expansions:

```json
{
  "dashboard": {
    "title": "PVC Chonker",
    "panels": [
      {
        "title": "PVC Usage",
        "type": "stat",
        "targets": [
          {
            "expr": "pvcchonker_pvc_usage_percent"
          }
        ]
      },
      {
        "title": "Expansion Events",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(pvcchonker_resizer_successful_resize_total[5m])"
          }
        ]
      }
    ]
  }
}
```

## Testing Examples

### Load Testing
Test PVC expansion under load:

```bash
#!/bin/bash
# fill-pvc.sh - Fill PVC to trigger expansion

PVC_NAME="test-pvc"
MOUNT_PATH="/data"

# Fill to 85% to trigger expansion
kubectl exec -it test-pod -- dd if=/dev/zero of=${MOUNT_PATH}/testfile bs=1M count=850

# Monitor expansion
kubectl get pvc ${PVC_NAME} -w
```

### Validation Scripts
Validate PVC Chonker configuration:

```bash
#!/bin/bash
# validate-config.sh

echo "Checking PVC Chonker installation..."

# Check operator
kubectl get pods -n pvc-chonker-system

# Check CRDs
kubectl get crd | grep pvc-chonker

# Check metrics
kubectl port-forward -n pvc-chonker-system svc/pvc-chonker-metrics 8080:8080 &
sleep 2
curl -s http://localhost:8080/metrics | grep pvcchonker || echo "Metrics not available"
kill %1

echo "Validation complete"
```

## Best Practices from Examples

1. **Start Conservative** - Begin with higher thresholds and smaller increases
2. **Monitor Closely** - Watch expansion events and adjust as needed
3. **Test Thoroughly** - Validate configuration in non-production first
4. **Plan for Growth** - Consider long-term storage requirements
5. **Use Policies** - Leverage PVCPolicy for consistent configuration
6. **Set Limits** - Always define maximum sizes to prevent runaway growth

## Contributing Examples

Have a useful PVC Chonker pattern? We'd love to include it!

1. Fork the repository
2. Add your example to the appropriate category
3. Include complete YAML manifests and explanations
4. Submit a pull request

## Next Steps

- Choose an example that matches your use case
- Adapt the configuration to your specific requirements
- Check the [Configuration Guide](../guides/configuration.md) for detailed options
- Set up [Monitoring](../guides/metrics.md) to track expansion events