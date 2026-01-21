package main

import (
	"testing"
)

func TestVersionDefaults(t *testing.T) {
	// Verify default values are set correctly
	if version != "dev" {
		t.Errorf("expected version to be 'dev', got: %s", version)
	}
	if gitHash != "unknown" {
		t.Errorf("expected gitHash to be 'unknown', got: %s", gitHash)
	}
}
