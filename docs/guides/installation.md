# Installation Guide

This guide covers installing PVC Chonker in your Kubernetes cluster using various methods.

## Prerequisites

- Kubernetes cluster (v1.19+) with CSI volume expansion support
- Kubelet metrics endpoint available (`/metrics`)
- Storage class with `allowVolumeExpansion: true`
- Helm 3.x (for Helm installation)
- **Cloud Authentication**: Proper IAM roles/service accounts for storage operations

## Helm Installation (Recommended)

### Add Repository

```bash
helm repo add logiciq https://logiciq.github.io/helm-charts
helm repo update
```

### Basic Installation

```bash
# Install with default settings
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace
```

### Advanced Installation

```bash
# Install with PVCGroup support (webhook enabled)
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set webhook.enabled=true \
  --set metrics.enabled=true
```

### Configuration Options

Create a `values.yaml` file:

```yaml
# values.yaml
replicaCount: 1

image:
  repository: logiciq/pvc-chonker
  tag: "v0.2.0"
  pullPolicy: IfNotPresent

# Service Account for Cloud Authentication
serviceAccount:
  create: true
  name: pvc-chonker
  annotations:
    # AWS EKS - AssumeRole with IRSA
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT:role/pvc-chonker-role"
    # GCP GKE - Workload Identity
    iam.gke.io/gcp-service-account: "pvc-chonker@PROJECT.iam.gserviceaccount.com"
    # Azure AKS - Workload Identity
    azure.workload.identity/client-id: "CLIENT-ID"

# AWS Configuration (alternative to IRSA)
aws:
  # Use AWS Access Keys (not recommended for production)
  credentials:
    secretName: aws-credentials  # Secret with AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
  region: us-west-2
  # Or use IAM role ARN for cross-account access
  roleArn: "arn:aws:iam::ACCOUNT:role/cross-account-role"

# Global defaults
config:
  threshold: 80.0
  increase: "10%"
  cooldown: "15m"
  minScaleUp: "1Gi"

# Webhook for PVCGroup support
webhook:
  enabled: true
  port: 9443

# Metrics
metrics:
  enabled: true
  port: 8080

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## Cloud Authentication Setup

PVC Chonker requires appropriate cloud permissions to expand storage volumes. Configure authentication using service account annotations.

### AWS Authentication Options

PVC Chonker supports multiple AWS authentication methods:

#### Option 1: IRSA (Recommended for EKS)
```bash
# Create IAM role with trust policy for OIDC
aws iam create-role --role-name pvc-chonker-role --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::ACCOUNT:oidc-provider/oidc.eks.REGION.amazonaws.com/id/OIDC_ID"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:sub": "system:serviceaccount:pvc-chonker-system:pvc-chonker",
        "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:aud": "sts.amazonaws.com"
      }
    }
  }]
}'

# Attach EBS CSI policy
aws iam attach-role-policy --role-name pvc-chonker-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/Amazon_EBS_CSI_Driver_Policy

# Install with IRSA
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::ACCOUNT:role/pvc-chonker-role"
```

#### Option 2: AWS Access Keys
```bash
# Create IAM user with programmatic access
aws iam create-user --user-name pvc-chonker-user

# Attach EBS CSI policy to user
aws iam attach-user-policy --user-name pvc-chonker-user \
  --policy-arn arn:aws:iam::aws:policy/service-role/Amazon_EBS_CSI_Driver_Policy

# Create access keys
aws iam create-access-key --user-name pvc-chonker-user

# Create Kubernetes secret with credentials
kubectl create secret generic aws-credentials -n pvc-chonker-system \
  --from-literal=AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE \
  --from-literal=AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Install with access keys
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set aws.credentials.secretName=aws-credentials \
  --set aws.region=us-west-2
```

#### Option 3: Cross-Account Role ARN
```bash
# For cross-account scenarios
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set aws.roleArn="arn:aws:iam::TARGET-ACCOUNT:role/pvc-chonker-cross-account-role" \
  --set aws.region=us-west-2
```

### AWS EKS with IRSA (IAM Roles for Service Accounts)

#### 1. Create IAM Role


### GCP GKE with Workload Identity

#### 1. Create GCP Service Account
```bash
# Create GCP service account
gcloud iam service-accounts create pvc-chonker \
  --display-name="PVC Chonker Service Account"

# Grant necessary permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:pvc-chonker@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/compute.storageAdmin"

# Enable Workload Identity binding
gcloud iam service-accounts add-iam-policy-binding \
  pvc-chonker@PROJECT_ID.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:PROJECT_ID.svc.id.goog[pvc-chonker-system/pvc-chonker]"
```

#### 2. Install with Workload Identity
```bash
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="pvc-chonker@PROJECT_ID.iam.gserviceaccount.com"
```

### Azure AKS with Workload Identity

#### 1. Create Azure Identity
```bash
# Create managed identity
az identity create --name pvc-chonker-identity --resource-group myResourceGroup

# Get client ID
CLIENT_ID=$(az identity show --name pvc-chonker-identity --resource-group myResourceGroup --query clientId -o tsv)

# Assign storage permissions
az role assignment create \
  --assignee $CLIENT_ID \
  --role "Storage Account Contributor" \
  --scope "/subscriptions/SUBSCRIPTION_ID/resourceGroups/myResourceGroup"
```

#### 2. Install with Workload Identity
```bash
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set serviceAccount.annotations."azure\.workload\.identity/client-id"="$CLIENT_ID"
```

### Alternative Authentication Methods

#### AWS Access Keys (Not Recommended for Production)
For development or legacy environments:
```bash
# Create AWS credentials secret
kubectl create secret generic aws-credentials -n pvc-chonker-system \
  --from-literal=AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE \
  --from-literal=AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Install with AWS credentials
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set aws.credentials.secretName=aws-credentials \
  --set aws.region=us-west-2
```

#### Node Instance Profiles (AWS)
For clusters where IRSA is not available:
```bash
# Attach EBS CSI policy to node instance profile
aws iam attach-role-policy --role-name NodeInstanceRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/Amazon_EBS_CSI_Driver_Policy
```

#### Service Account Keys (GCP)
**Not recommended for production:**
```bash
# Create and download service account key
gcloud iam service-accounts keys create key.json \
  --iam-account=pvc-chonker@PROJECT_ID.iam.gserviceaccount.com

# Create Kubernetes secret
kubectl create secret generic gcp-credentials \
  --from-file=key.json=key.json -n pvc-chonker-system

# Mount secret in deployment
helm install pvc-chonker logiciq/pvc-chonker -n pvc-chonker-system --create-namespace \
  --set volumes[0].name=gcp-credentials \
  --set volumes[0].secret.secretName=gcp-credentials \
  --set volumeMounts[0].name=gcp-credentials \
  --set volumeMounts[0].mountPath=/var/secrets/google
```

## Manual Installation

### Install CRDs

```bash
kubectl apply -f https://raw.githubusercontent.com/LogicIQ/pvc-chonker/main/config/crd/bases/pvc-chonker.io_pvcpolicies.yaml
kubectl apply -f https://raw.githubusercontent.com/LogicIQ/pvc-chonker/main/config/crd/bases/pvc-chonker.io_pvcgroups.yaml
```

### Install Operator

```bash
kubectl apply -f https://raw.githubusercontent.com/LogicIQ/pvc-chonker/main/config/rbac/
kubectl apply -f https://raw.githubusercontent.com/LogicIQ/pvc-chonker/main/config/manager/
```

## Verification

### Check Installation

```bash
# Check operator pods
kubectl get pods -n pvc-chonker-system

# Check CRDs
kubectl get crd | grep pvc-chonker

# Check metrics endpoint
kubectl port-forward -n pvc-chonker-system svc/pvc-chonker-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Test Basic Functionality

```bash
# Create test PVC
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
  annotations:
    pvc-chonker.io/enabled: "true"
    pvc-chonker.io/threshold: "80%"
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
  storageClassName: your-storage-class
EOF

# Check logs
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager
```

## Storage Class Requirements

Ensure your storage class supports volume expansion:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: expandable-storage
provisioner: your-csi-driver
allowVolumeExpansion: true  # Required
parameters:
  type: gp3
```

## Kubelet Metrics

### Managed Clusters
Most managed Kubernetes services (EKS, GKE, AKS) have kubelet metrics enabled by default.

### Self-Managed Clusters
You may need to configure kubelet to expose metrics:

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

### Verification
Check if metrics are available:

```bash
# From within cluster
curl http://node-ip:10255/metrics | grep kubelet_volume

# Alternative port (may require authentication)
curl -k https://node-ip:10250/metrics
```

## RBAC Configuration

PVC Chonker requires specific Kubernetes permissions and cloud storage permissions:

### Kubernetes RBAC
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pvc-chonker-manager
rules:
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["pvc-chonker.io"]
  resources: ["pvcpolicies", "pvcgroups"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

### Cloud Storage Permissions

#### AWS Permissions
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeVolumes",
        "ec2:ModifyVolume",
        "ec2:DescribeVolumesModifications"
      ],
      "Resource": "*"
    }
  ]
}
```

#### GCP Permissions
```bash
# Required roles
roles/compute.storageAdmin
# Or specific permissions:
# compute.disks.get
# compute.disks.resize
# compute.disks.list
```

#### Azure Permissions
```bash
# Required roles
"Storage Account Contributor"
# Or specific permissions:
# Microsoft.Compute/disks/read
# Microsoft.Compute/disks/write
```

## Troubleshooting

### Common Issues

**Operator not starting:**
```bash
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager
kubectl describe pod -n pvc-chonker-system
```

**Authentication issues:**
```bash
# Check service account annotations
kubectl get sa pvc-chonker -n pvc-chonker-system -o yaml

# Test cloud permissions
# AWS: Check CloudTrail for AssumeRole events
# GCP: Check IAM audit logs
# Azure: Check Activity Log for authentication
```

**Metrics not available:**
```bash
# Check kubelet metrics
kubectl get --raw /api/v1/nodes/node-name/proxy/metrics
```

**PVC not expanding:**
```bash
# Check PVC events
kubectl describe pvc your-pvc

# Check operator logs
kubectl logs -n pvc-chonker-system deployment/pvc-chonker-controller-manager

# Verify storage class supports expansion
kubectl get storageclass your-storage-class -o yaml | grep allowVolumeExpansion
```

## Next Steps

- **[Quick Start](./quick-start.md)** - Try basic examples
- **[Configuration](./configuration.md)** - Understand configuration options
- **[Metrics](./metrics.md)** - Set up monitoring
- **[Examples](../examples/overview.md)** - Real-world usage patterns