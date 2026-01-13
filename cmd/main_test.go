package main

import (
	"testing"
)

func TestVersionVariables(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
	if gitHash == "" {
		t.Error("gitHash should not be empty")
	}
}

func TestVersionDefaults(t *testing.T) {
	// Test that default values are set when not injected via ldflags
	if version == "" {
		t.Error("version should not be empty, expected default value 'dev'")
	}
	if gitHash == "" {
		t.Error("gitHash should not be empty, expected default value 'unknown'")
	}

	// Verify default values are reasonable
	if version != "dev" && version == "" {
		t.Logf("version is set to: %s (not default 'dev')", version)
	}
	if gitHash != "unknown" && gitHash == "" {
		t.Logf("gitHash is set to: %s (not default 'unknown')", gitHash)
	}
}
