package main

import (
	"testing"
)

func TestVersionDefaults(t *testing.T) {
	// Test that default values are set when not injected via ldflags
	if version == "" {
		t.Error("version should not be empty, expected default value 'dev'")
	}
	if gitHash == "" {
		t.Error("gitHash should not be empty, expected default value 'unknown'")
	}

	// Verify default values
	if version != "dev" {
		t.Logf("version is set to: %s (not default 'dev')", version)
	}
	if gitHash != "unknown" {
		t.Logf("gitHash is set to: %s (not default 'unknown')", gitHash)
	}
}
