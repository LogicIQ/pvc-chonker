# Integration Tests

Integration tests using envtest (controller-runtime's test framework) with mocked kubelet metrics.

## What's Tested

### Annotations Integration (`annotations_test.go`)
- PVC annotation parsing with various configurations
- Size calculation logic (percentage and fixed amounts)
- Cooldown period management
- Maximum size validation
- Error handling for invalid annotations

### Controller Integration (`controller_test.go`)
- PVC eligibility checking
- Storage class validation
- PVC expansion logic
- Dry run mode behavior
- Cooldown enforcement
- Mock kubelet metrics integration

## Running Tests

### Prerequisites
```bash
# Install ginkgo
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

### Run Tests
```bash
# Run integration tests
task test-integration

# Direct from integration directory
cd test/integration
task run
```

## Test Environment

**envtest** provides:
- Real etcd and kube-apiserver
- Fast test execution
- Mocked kubelet metrics
- No container runtime needed

## Test Structure

- **`suite_test.go`**: Test suite setup and teardown
- **`annotations_test.go`**: Annotation parsing and logic tests
- **`controller_test.go`**: Controller behavior and integration tests

## Mock Components

- **MockMetricsCollector**: Simulates kubelet volume metrics
- **FakeRecorder**: Records Kubernetes events for testing

## Test Data

Tests create:
- Test namespace: `pvc-chonker-test`
- Expandable storage class: `expandable-storage`
- Non-expandable storage class: `non-expandable-storage`
- Various test PVCs with different annotation configurations

## Coverage

These integration tests cover:
- ✅ End-to-end annotation processing
- ✅ Controller decision logic
- ✅ PVC expansion workflows
- ✅ Error conditions and edge cases
- ✅ Dry run mode validation
- ✅ Storage class compatibility

Fast integration testing focused on annotation processing and controller logic. For full end-to-end testing with real storage, see `test/e2e/`.