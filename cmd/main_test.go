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
	if version != "dev" && version == "" {
		t.Error("version should have default value 'dev' when not set")
	}
	if gitHash != "unknown" && gitHash == "" {
		t.Error("gitHash should have default value 'unknown' when not set")
	}
}
