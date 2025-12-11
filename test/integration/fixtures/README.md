# Test Fixtures

This directory contains YAML fixtures used by integration tests.

## Files

- **`namespace.yaml`** - Test namespace for integration tests
- **`storage-classes.yaml`** - Expandable and non-expandable storage classes
- **`test-pvcs.yaml`** - Sample PVCs with different annotation configurations
- **`loader.go`** - Utility functions for loading YAML fixtures

## Usage

```go
// Load single fixture
obj, err := fixtures.LoadFixture("namespace.yaml")

// Load and create single fixture
err := fixtures.LoadAndCreateFixture(ctx, k8sClient, "namespace.yaml")

// Load multiple fixtures from one file
objects, err := fixtures.LoadMultipleFixtures("storage-classes.yaml")

// Load and create multiple fixtures
err := fixtures.LoadAndCreateMultipleFixtures(ctx, k8sClient, "storage-classes.yaml")
```

## Test PVC Examples

The `test-pvcs.yaml` contains examples of:
- **enabled-pvc**: Basic enabled PVC with default settings
- **custom-config-pvc**: PVC with custom annotation values
- **disabled-pvc**: Disabled PVC for negative testing