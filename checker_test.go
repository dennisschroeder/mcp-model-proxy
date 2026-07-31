package main

import (
	"strings"
	"testing"
)

// testChecker builds a ToolChecker with synthetic tools, so tests don't
// depend on which real CLIs happen to be installed on the machine running them.
func testChecker() *ToolChecker {
	return &ToolChecker{
		tools: map[string]*Tool{
			"present": {
				Name:         "Present Tool",
				BinaryName:   "true", // always exits 0
				VersionCmd:   []string{"true"},
				Installation: "no install needed",
			},
			"absent": {
				Name:         "Absent Tool",
				BinaryName:   "definitely-not-a-real-binary-xyz",
				VersionCmd:   []string{"definitely-not-a-real-binary-xyz", "--version"},
				Installation: "Install Absent Tool:\n  brew install absent-tool",
			},
		},
	}
}

func TestIsAvailable(t *testing.T) {
	tc := testChecker()

	if !tc.IsAvailable("present") {
		t.Error(`IsAvailable("present") = false, want true`)
	}
	if tc.IsAvailable("absent") {
		t.Error(`IsAvailable("absent") = true, want false`)
	}
	if tc.IsAvailable("unknown-key") {
		t.Error(`IsAvailable("unknown-key") = true, want false`)
	}
}

func TestUnavailableMessage(t *testing.T) {
	tc := testChecker()

	msg := tc.UnavailableMessage("absent")
	if !strings.Contains(msg, "Absent Tool") || !strings.Contains(msg, "brew install absent-tool") {
		t.Errorf("UnavailableMessage(%q) = %q, want it to include the tool's Name and Installation text", "absent", msg)
	}

	fallback := tc.UnavailableMessage("unknown-key")
	if !strings.Contains(fallback, "unknown-key") {
		t.Errorf("UnavailableMessage(%q) = %q, want it to mention the key", "unknown-key", fallback)
	}
}
