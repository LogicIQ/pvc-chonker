# Troubleshooting Guide

This guide helps diagnose and resolve common PVC Chonker issues.

## Common Issues

### PVC Not Expanding

#### Symptoms
- PVC usage above threshold but no expansion occurs
- No expansion events in PVC description
- Controller logs show no activity for the PVC

#### Diagnosis Steps
```bash
# 1. Check PVC annotations
kubectl get pvc your-pvc -o yaml | grep -A 10 annotations

# 2. Check PVC events
kubectl describe pvc your-pvc

# 3. Check controller logs
kubectl logs -n pvc-chonker-system deployment/controller-manager --tail=50

# 4. Check storage class
kubectl get storageclass your-storage-class -o yaml
```

#### Common Causes & Solutions

**PVC not enabled:**
```yaml
# Solution: Add enable annotation
metadata:
  annotations:
    pvc-chonker.io/enabled: "true"
```

**Storage class doesn't support expansion:**
```yaml
# Check allowVolumeExpansion
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: your-storage-class
allowVolumeExpansion: true  # Must be true
```

**PVC in cooldown period:**
```bash
# Check last expansion time
kubectl get pvc your-pvc -o jsonpath='{.metadata.annotations.pvc-chonker\.io/last-expansion}'

# Solution: Wait for cooldown or reduce cooldown period
```

**Maximum size reached:**
```bash
# Check current size vs max-size annotation
kubectl get pvc your-pvc -o jsonpath='{.spec.resources.requests.storage}'
kubectl get pvc your-pvc -o jsonpath='{.metadata.annotations.pvc-chonker\.io/max-size}'
```

### Controller Not Starting

#### Symptoms
- Controller pods in CrashLoopBackOff
- Controller pods not ready
- No controller logs

#### Diagnosis Steps
```bash
# Check pod status
kubectl get pods -n pvc-chonker-system

# Check pod events
kubectl describe pod -n pvc-chonker-system controller-manager-xxx

# Check pod logs
kubectl logs -n pvc-chonker-system controller-manager-xxx
```

#### Common Causes & Solutions

**RBAC permissions missing:**
```bash
# Check ClusterRole exists
kubectl get clusterrole pvc-chonker-manager-role

# Check ClusterRoleBinding
kubectl get clusterrolebinding pvc-chonker-manager-rolebinding
```

**CRDs not installed:**
```bash
# Check CRDs exist
kubectl get crd | grep pvc-chonker

# Install missing CRDs
kubectl apply -f config/crd/bases/
```

**Image pull issues:**
```bash
# Check image pull policy and availability
kubectl describe pod -n pvc-chonker-system controller-manager-xxx | grep -A 5 Events
```

### Metrics Not Available

#### Symptoms
- Kubelet metrics endpoint returns 404
- Controller logs show "metrics not found" errors
- PVCs not expanding despite being above threshold

#### Diagnosis Steps
```bash
# 1. Check kubelet metrics endpoint
kubectl get nodes -o wide
# Then test: curl http://NODE-IP:10255/metrics

# 2. Check alternative kubelet port
curl -k https://NODE-IP:10250/metrics

# 3. Check from within cluster
kubectl run debug --image=curlimages/curl -it --rm -- curl http://NODE-IP:10255/metrics
```

#### Solutions

**Enable kubelet metrics (self-managed clusters):**
```yaml
# kubelet-config.yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serverTLSBootstrap: true
authentication:
  webhook:
    enabled: true
authorization:
  mode: Webhook
```

**Configure alternative kubelet URL:**
```bash
# Use environment variable or flag
--kubelet-url="https://NODE-IP:10250"
```

### Webhook Issues (PVCGroup)

#### Symptoms
- PVC creation fails with webhook errors
- Webhook timeout errors
- PVCGroup annotations not applied

#### Diagnosis Steps
```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration pvc-chonker-mutating-webhook-configuration

# Check webhook service
kubectl get svc -n pvc-chonker-system pvc-chonker-webhook-service

# Check webhook certificates
kubectl get secret -n pvc-chonker-system pvc-chonker-webhook-server-cert

# Check controller logs for webhook errors
kubectl logs -n pvc-chonker-system deployment/controller-manager | grep webhook
```

#### Solutions

**Webhook not enabled:**
```bash
# Enable webhook in controller
--enable-webhook=true
# Or environment variable
PVC_CHONKER_ENABLE_WEBHOOK=true
```

**Certificate issues:**
```bash
# Regenerate webhook certificates
./hack/generate-webhook-certs.sh
kubectl apply -f config/webhook/
```

**Service connectivity:**
```bash
# Check service endpoints
kubectl get endpoints -n pvc-chonker-system pvc-chonker-webhook-service

# Test webhook connectivity
kubectl run debug --image=curlimages/curl -it --rm -- \
  curl -k https://pvc-chonker-webhook-service.pvc-chonker-system.svc:443/mutate--v1-persistentvolumeclaim
```

## Performance Issues

### Slow Expansion Detection

#### Symptoms
- Long delays between threshold breach and expansion
- High controller CPU usage
- Many PVCs not being processed

#### Diagnosis
```bash
# Check reconciliation interval
kubectl logs -n pvc-chonker-system deployment/controller-manager | grep "watch-interval"

# Check controller resource usage
kubectl top pod -n pvc-chonker-system

# Check number of managed PVCs
kubectl get pvc --all-namespaces -o json | jq '.items | map(select(.metadata.annotations."pvc-chonker.io/enabled" == "true")) | length'
```

#### Solutions

**Reduce watch interval:**
```bash
# Faster reconciliation (higher CPU usage)
--watch-interval=30s
```

**Increase controller resources:**
```yaml
resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi
```

**Reduce managed PVCs:**
```bash
# Disable expansion for unused PVCs
kubectl annotate pvc unused-pvc pvc-chonker.io/enabled=false
```

### High Memory Usage

#### Symptoms
- Controller OOMKilled
- High memory usage in metrics
- Slow API responses

#### Solutions

**Increase memory limits:**
```yaml
resources:
  limits:
    memory: 2Gi
  requests:
    memory: 512Mi
```

**Reduce concurrent operations:**
```bash
--max-parallel=2  # Default is 4
```

## Configuration Issues

### Policy Not Applied

#### Symptoms
- PVC matches policy selector but settings not applied
- Policy exists but PVC uses different configuration

#### Diagnosis
```bash
# Check policy selector matches PVC labels
kubectl get pvcpolicy your-policy -o yaml
kubectl get pvc your-pvc -o yaml | grep -A 10 labels

# Check policy namespace matches PVC namespace
kubectl get pvcpolicy -n your-namespace

# Check for annotation overrides
kubectl get pvc your-pvc -o yaml | grep -A 20 annotations
```

#### Solutions

**Fix label matching:**
```yaml
# Ensure PVC labels match policy selector
metadata:
  labels:
    workload: database  # Must match policy selector
```

**Check namespace:**
```bash
# PVCPolicy must be in same namespace as PVC
kubectl get pvcpolicy -n correct-namespace
```

### Group Coordination Issues

#### Symptoms
- PVCs in group have different sizes
- Group coordination not working
- Webhook not applying group settings

#### Diagnosis
```bash
# Check PVCGroup status
kubectl get pvcgroup your-group -o yaml

# Check group member PVCs
kubectl get pvc -l app=your-app -o custom-columns=NAME:.metadata.name,SIZE:.spec.resources.requests.storage

# Check webhook logs
kubectl logs -n pvc-chonker-system deployment/controller-manager | grep "webhook\|group"
```

#### Solutions

**Verify webhook is enabled:**
```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration pvc-chonker-mutating-webhook-configuration

# Enable webhook if missing
helm upgrade pvc-chonker logiciq/pvc-chonker --set webhook.enabled=true
```

**Check group selector:**
```yaml
# Ensure PVC labels match group selector
spec:
  selector:
    matchLabels:
      app: elasticsearch  # PVCs must have this label
```

## Debugging Commands

### Comprehensive Status Check
```bash
#!/bin/bash
echo "=== PVC Chonker Status ==="
echo "Controller Pods:"
kubectl get pods -n pvc-chonker-system

echo -e "\nController Logs (last 20 lines):"
kubectl logs -n pvc-chonker-system deployment/controller-manager --tail=20

echo -e "\nManaged PVCs:"
kubectl get pvc --all-namespaces -o json | \
  jq -r '.items[] | select(.metadata.annotations."pvc-chonker.io/enabled" == "true") | "\(.metadata.namespace)/\(.metadata.name)"'

echo -e "\nPVCPolicies:"
kubectl get pvcpolicy --all-namespaces

echo -e "\nPVCGroups:"
kubectl get pvcgroup --all-namespaces

echo -e "\nWebhook Configuration:"
kubectl get mutatingwebhookconfiguration pvc-chonker-mutating-webhook-configuration 2>/dev/null || echo "Webhook not configured"
```

### PVC Expansion History
```bash
#!/bin/bash
PVC_NAME=$1
NAMESPACE=${2:-default}

echo "=== PVC Expansion History: $NAMESPACE/$PVC_NAME ==="
kubectl get events --field-selector involvedObject.name=$PVC_NAME -n $NAMESPACE --sort-by='.firstTimestamp'

echo -e "\nCurrent PVC Status:"
kubectl get pvc $PVC_NAME -n $NAMESPACE -o yaml | grep -A 20 -B 5 "pvc-chonker"

echo -e "\nController Logs for this PVC:"
kubectl logs -n pvc-chonker-system deployment/controller-manager | grep $PVC_NAME
```

### Metrics Validation
```bash
#!/bin/bash
echo "=== Kubelet Metrics Validation ==="
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "Testing node: $NODE_NAME"

# Test metrics endpoint
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/metrics | grep kubelet_volume_stats | head -5 || echo "No volume metrics found"

echo -e "\nPVC Chonker Metrics:"
kubectl port-forward -n pvc-chonker-system svc/pvc-chonker-metrics 8080:8080 &
PF_PID=$!
sleep 2
curl -s http://localhost:8080/metrics | grep pvcchonker | head -10 || echo "No PVC Chonker metrics found"
kill $PF_PID 2>/dev/null
```

## Getting Help

### Collecting Debug Information
```bash
#!/bin/bash
echo "Collecting PVC Chonker debug information..."
mkdir -p pvc-chonker-debug

# Controller information
kubectl get pods -n pvc-chonker-system -o yaml > pvc-chonker-debug/controller-pods.yaml
kubectl logs -n pvc-chonker-system deployment/controller-manager > pvc-chonker-debug/controller-logs.txt

# Configuration
kubectl get pvcpolicy --all-namespaces -o yaml > pvc-chonker-debug/pvcpolicies.yaml
kubectl get pvcgroup --all-namespaces -o yaml > pvc-chonker-debug/pvcgroups.yaml
kubectl get mutatingwebhookconfiguration pvc-chonker-mutating-webhook-configuration -o yaml > pvc-chonker-debug/webhook-config.yaml 2>/dev/null

# Managed PVCs
kubectl get pvc --all-namespaces -o yaml | \
  yq eval 'select(.metadata.annotations."pvc-chonker.io/enabled" == "true")' > pvc-chonker-debug/managed-pvcs.yaml

# System information
kubectl version > pvc-chonker-debug/cluster-version.txt
kubectl get nodes -o yaml > pvc-chonker-debug/nodes.yaml
kubectl get storageclass -o yaml > pvc-chonker-debug/storageclasses.yaml

echo "Debug information collected in pvc-chonker-debug/"
echo "Please attach this directory when reporting issues."
```

### Support Channels
- **GitHub Issues**: [Report bugs](https://github.com/LogicIQ/pvc-chonker/issues)
- **GitHub Discussions**: [Community support](https://github.com/LogicIQ/pvc-chonker/discussions)
- **Documentation**: [Project docs](https://logiciq.ca/pvc-chonker)