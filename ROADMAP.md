# PVC-Chonker Kubernetes Operator Roadmap

## Project Overview

**pvc-chonker** is a cloud-agnostic Kubernetes operator for automatic PVC expansion with modular cloud provider support. Built with operator-sdk, it provides intelligent storage management without external dependencies like DynamoDB.

## Core Design Principles

1. **Cloud Agnostic**: Modular architecture supporting multiple cloud providers
2. **No External Dependencies**: Self-contained operation without external databases
3. **Annotation-Based**: Configuration through Kubernetes annotations (no CRDs initially)
4. **Monitoring First**: Comprehensive metrics and observability
5. **Extensible**: Plugin architecture for cloud provider support

## Phase 1: Annotation System & Core Controller

### 1.1 Annotation System
Define comprehensive annotation schema:
```yaml
# Core annotations
pvc-chonker.io/enabled: "true"                    # Enable auto-expansion
pvc-chonker.io/policy: "database-policy"          # Reference to PVCPolicy
pvc-chonker.io/group: "db-cluster"                # Reference to PVCGroup
pvc-chonker.io/threshold: "80%"                   # Storage threshold (override)
pvc-chonker.io/inodes-threshold: "80%"            # Inode threshold (override)
pvc-chonker.io/increase: "10%"                    # Expansion amount (override)
pvc-chonker.io/max-size: "1000Gi"                 # Maximum size limit (override)

# Cloud provider specific
pvc-chonker.io/provider: "aws"                    # Cloud provider
pvc-chonker.io/aws-volume-type: "gp3"             # AWS specific config
```

### 1.2 Core Controller Implementation
- [ ] **PVC Controller**: Main reconciliation loop
- [ ] **Metrics Collection**: Kubelet metrics integration
- [ ] **Expansion Logic**: Core resizing algorithm
  - Threshold-based expansion (storage + inodes)
  - Configurable increase amounts (percentage/fixed)
  - Maximum size limits enforcement

### 1.3 AWS Provider Implementation
- [ ] **AWS Interface**: Implement cloud provider interface
- [ ] **EBS Integration**: Basic EBS volume support
- [ ] **IAM Integration**: Proper AWS permissions handling

### 1.4 Metrics & Monitoring
- [ ] **Prometheus Metrics**: Comprehensive metric collection
  ```
  pvc_chonker_expansions_total{pvc, namespace, provider}
  pvc_chonker_expansion_failures_total{pvc, namespace, reason}
  pvc_chonker_loop_duration_seconds
  pvc_chonker_threshold_reached_total{pvc, namespace, type}
  pvc_chonker_max_size_reached_total{pvc, namespace}
  pvc_chonker_pvc_unhealthy_total{pvc, namespace}
  ```
- [ ] **Health Checks**: Readiness and liveness probes
- [ ] **Event Recording**: Kubernetes events for operations

## Phase 2: Helm Chart & Basic Features

### 2.1 Helm Chart
- [ ] **Production-ready Helm deployment**
- [ ] **RBAC Templates**: Minimal privilege configurations
- [ ] **Configuration values**: Configurable deployment options

### 2.2 CRD Controllers
- [ ] **PVCPolicy Controller**: Manage PVC policy lifecycle
- [ ] **PVCGroup Controller**: Manage group coordination

### 2.3 Mutating Admission Webhook
- [ ] **Webhook Implementation**: PVC creation interception
- [ ] **TLS Management**: Automatic certificate handling

## Phase 3: Enhanced Features & Reliability

### 3.1 Reliability & Safety
- [ ] **Expansion Cooldown**: Prevent rapid successive expansions
- [ ] **Rate Limiting**: Limit concurrent expansion operations
- [ ] **Dry Run Mode**: Test expansion logic without changes
- [ ] **Circuit Breaker**: Fail-safe mechanisms for provider errors

### 3.2 Enhanced Monitoring
- [ ] **Grafana Dashboard**: Pre-built monitoring dashboard
- [ ] **Alert Rules**: Prometheus alerting rules
- [ ] **Structured Logging**: JSON logging with correlation IDs

### 3.3 Testing Framework
- [ ] **Unit Tests**: Comprehensive test coverage (>80%)
- [ ] **Integration Tests**: End-to-end testing with kind/minikube
- [ ] **Cloud Provider Tests**: AWS integration testing

## Phase 4: GCP Support

### 4.1 GCP Provider Implementation
- [ ] **GCP Interface**: Implement GCP provider
- [ ] **Persistent Disk Support**: Basic GCP disk support
- [ ] **GKE Integration**: Google Kubernetes Engine specific features
- [ ] **IAM & Service Accounts**: GCP authentication handling

## Phase 5: Production Readiness

### 5.1 Operational Features
- [ ] **Security Scanning**: Container and code security

### 5.2 Documentation & Examples
- [ ] **Operator Documentation**: Complete usage guide
- [ ] **Cloud Provider Guides**: Provider-specific setup
- [ ] **Troubleshooting Guide**: Common issues and solutions
- [ ] **Example Configurations**: Real-world use cases

## Technical Architecture

### Cloud Provider Interface
```go
type CloudProvider interface {
    // Volume operations
    CanExpand(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (bool, error)
    GetVolumeConstraints(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (*VolumeConstraints, error)
    ValidateExpansion(ctx context.Context, pvc *corev1.PersistentVolumeClaim, newSize resource.Quantity) error
    
    // Provider info
    Name() string
    SupportedStorageClasses() []string
    DefaultConfiguration() *ProviderConfig
}

type VolumeConstraints struct {
    MinSize     resource.Quantity
    MaxSize     resource.Quantity
    StepSize    resource.Quantity
    VolumeType  string
}
```

### Configuration Management
```go
type ChonkerConfig struct {
    // Global settings
    WatchInterval     time.Duration
    DefaultThreshold  string
    DefaultIncrease   string
    MinScaleUp        resource.Quantity
    
    // Provider settings
    Provider          string
    ProviderConfig    map[string]interface{}
    
    // Feature flags
    EnableWebhook     bool
    EnableMetrics     bool
    DryRun           bool
}
```

## Key Features

### Core Capabilities
1. **No External Dependencies**: Self-contained operation
2. **Multi-Cloud Support**: Pluggable cloud provider architecture
3. **Comprehensive Monitoring**: Enhanced metrics and observability
4. **Modular Design**: Clean separation of concerns
5. **Production Ready**: Comprehensive testing and documentation
6. **Security First**: Minimal RBAC and security scanning

### Advanced Features
- **CRD-Based Configuration**: Kubernetes-native policy management
- **Group Coordination**: PVCGroup CRD for cross-PVC coordination
- **Mutating Webhook**: Policy-driven PVC creation interception
- **Provider Abstraction**: Clean cloud provider interface
- **Configuration Management**: Flexible configuration options
- **Reliability**: Circuit breakers and safety mechanisms
- **Observability**: Structured logging and comprehensive metrics

## Success Metrics

### Phase 1 Success Criteria
- [ ] Successfully expand AWS EBS volumes based on thresholds
- [ ] Handle 50+ PVCs in test cluster
- [ ] Complete metrics integration with Prometheus
- [ ] Pass all unit and integration tests

### Phase 4 Success Criteria  
- [ ] Support both AWS and GCP in production
- [ ] Handle multiple PVCs across cloud providers
- [ ] Complete documentation and examples

### Production Readiness Criteria
- [ ] Stable operation in production environments
- [ ] Fast expansion decision making
- [ ] Comprehensive monitoring and alerting
- [ ] Security audit compliance

## Risk Mitigation

### Technical Risks
- **Kubelet Metrics Dependency**: Fallback mechanisms for metrics unavailability
- **Cloud Provider API Limits**: Rate limiting and retry logic
- **Large Cluster Performance**: Efficient resource usage and caching

### Operational Risks
- **Runaway Expansion**: Multiple safety mechanisms and limits
- **Provider Outages**: Graceful degradation and error handling
- **Configuration Errors**: Validation and dry-run capabilities

## Future Enhancements (Post v1.0)

### Additional Cloud Providers
- [ ] **Azure Support**: Azure Disk integration
- [ ] **On-Premises**: Support for local storage providers

### Advanced Features
- [ ] **Multi-Cluster**: Cross-cluster PVC management
- [ ] **Cost Optimization**: Volume type recommendations based on usage patterns
- [ ] **Backup Integration**: Coordinate with backup solutions

### CRD API Examples

**PVCPolicy for Database Workloads:**
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
    initialSize: "100Gi"
    threshold: "85%"
    increase: "25%"
    maxSize: "2000Gi"
    minIncrease: "50Gi"
    provider: "aws"
    providerConfig:
      volumeType: "gp3"
      iops: 3000
```

**PVCGroup for Coordinated Sizing:**
```yaml
apiVersion: pvc-chonker.io/v1alpha1
kind: PVCGroup
metadata:
  name: elasticsearch-cluster
spec:
  groupName: "es-data"
  coordinationPolicy: "largest"
  members:
  - namespace: "elastic"
    name: "data-es-master-0"
  - namespace: "elastic"
    name: "data-es-master-1"
  status:
    currentSize: "500Gi"
    lastExpansion: "2024-01-15T10:30:00Z"
    expansionHistory:
    - timestamp: "2024-01-15T10:30:00Z"
      fromSize: "400Gi"
      toSize: "500Gi"
      reason: "threshold_reached"
```

This roadmap provides a comprehensive path to building a production-ready, cloud-agnostic PVC expansion operator with advanced policy management and multi-cloud support.