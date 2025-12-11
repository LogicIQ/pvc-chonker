# PVC-Chonker Kubernetes Operator Roadmap

## Project Overview

**pvc-chonker** is a cloud-agnostic Kubernetes operator for automatic PVC expansion. Built with operator-sdk, it provides intelligent storage management without external dependencies, relying purely on Kubernetes CSI for volume expansion.

## Core Design Principles

1. **Cloud Agnostic**: Works with any CSI-compatible storage
2. **No External Dependencies**: Self-contained operation without external databases
3. **Annotation-Based**: Simple configuration through Kubernetes annotations
4. **Safety First**: Cooldown protection and resize detection
5. **Configurable**: Global defaults with per-PVC overrides

## Phase 1: Annotation System & Core Controller

### 1.1 Annotation System ✅ COMPLETED
```yaml
pvc-chonker.io/enabled: "true"                    # Enable auto-expansion
pvc-chonker.io/threshold: "80%"                   # Storage threshold
pvc-chonker.io/increase: "10%"                    # Expansion amount
pvc-chonker.io/max-size: "1000Gi"                 # Maximum size limit
pvc-chonker.io/min-scale-up: "1Gi"                # Minimum expansion amount
pvc-chonker.io/cooldown: "15m"                    # Cooldown between expansions
```

### 1.2 Core Controller Implementation ✅ COMPLETED
- [x] **PVC Controller**: Main reconciliation loop ✅
- [x] **Metrics Collection**: Kubelet metrics integration ✅
- [x] **Expansion Logic**: Core resizing algorithm ✅
  - [x] Threshold-based expansion (storage usage)
  - [x] Configurable increase amounts (percentage/fixed)
  - [x] Maximum size limits enforcement
  - [x] Cooldown period enforcement
  - [x] Minimum scale-up enforcement
  - [x] GiB boundary rounding
  - [x] Resize operation detection

### 1.3 Cloud-Agnostic Implementation
- [ ] **Storage Class Validation**: Check allowVolumeExpansion capability
- [ ] **Generic PVC Expansion**: Use Kubernetes CSI for all expansions
- [ ] **Error Handling**: Handle expansion failures gracefully
- [x] **Safety Checks**: Prevent concurrent operations ✅

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



## Phase 5: Production Readiness

### 5.1 Operational Features
- [ ] **Security Scanning**: Container and code security

### 5.2 Documentation & Examples
- [ ] **Operator Documentation**: Complete usage guide
- [ ] **Cloud Provider Guides**: Provider-specific setup
- [ ] **Troubleshooting Guide**: Common issues and solutions
- [ ] **Example Configurations**: Real-world use cases

## Technical Architecture ✅ COMPLETED

### Annotation Configuration
```go
type PVCConfig struct {
    Enabled       bool
    Threshold     float64
    Increase      string
    MaxSize       resource.Quantity
    Cooldown      time.Duration
    MinScaleUp    resource.Quantity
    LastExpansion *time.Time
}
```

### Global Configuration
```go
type GlobalConfig struct {
    Threshold  float64
    Increase   string
    Cooldown   time.Duration
    MinScaleUp resource.Quantity
    MaxSize    resource.Quantity
}
```

## Key Features

### Core Capabilities
1. **No External Dependencies**: Self-contained operation
2. **Cloud Agnostic**: Works with any CSI-compatible storage
3. **Safety First**: Cooldown protection and resize detection
4. **Simple Configuration**: Annotation-based with global defaults
5. **Flexible Expansion**: Percentage or fixed size increases
6. **Production Ready**: Comprehensive safety mechanisms

### Implemented Features ✅
- **Annotation System**: Complete configuration via annotations
- **Cooldown Management**: Prevents rapid successive expansions
- **Size Validation**: Respects maximum size limits and minimum increases
- **Resize Detection**: Skips PVCs currently being resized
- **GiB Rounding**: Clean storage boundaries
- **Global Defaults**: Configurable via flags/env vars

## Success Metrics

### Phase 1 Success Criteria
- [x] Annotation system implementation ✅
- [x] Expansion logic with safety features ✅
- [x] PVC controller implementation ✅
- [x] Metrics collection integration ✅
- [ ] Unit and integration tests

### Phase 2 Success Criteria  
- [ ] Support any CSI-compatible storage in production
- [ ] Handle multiple PVCs across different storage classes
- [ ] Complete documentation and examples
- [ ] Helm chart for deployment

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