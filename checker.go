package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// ToolChecker validates availability of required CLI tools
type ToolChecker struct {
	tools map[string]*Tool
}

// Tool represents a CLI tool and how to check it
type Tool struct {
	Name         string
	BinaryName   string
	VersionCmd   []string // command to check version/availability
	Installation string   // installation instructions
}

// NewToolChecker creates a new tool checker
func NewToolChecker() *ToolChecker {
	return &ToolChecker{
		tools: map[string]*Tool{
			"antigravity": {
				Name:       "Antigravity CLI",
				BinaryName: "agy",
				VersionCmd: []string{"agy", "--version"},
				Installation: `Install Antigravity CLI:
  macOS: brew install antigravity-cli
Then run: agy install
(installs as the "agy" binary, not "antigravity-cli")`,
			},
			"claude": {
				Name:       "Claude CLI",
				BinaryName: "claude",
				VersionCmd: []string{"claude", "--version"},
				Installation: `Install Claude CLI:
  npm install -g @anthropic-ai/claude-code
Then run: claude (to authenticate interactively)`,
			},
			"codex": {
				Name:       "Codex CLI",
				BinaryName: "codex",
				VersionCmd: []string{"codex", "--version"},
				Installation: `Install Codex CLI:
  macOS: brew install --cask codex
Then run: codex login`,
			},
		},
	}
}

// ValidateAll checks all tools and returns aggregated errors
func (tc *ToolChecker) ValidateAll() error {
	var errors []string

	for _, tool := range tc.tools {
		if err := tc.checkTool(tool); err != nil {
			errors = append(errors, fmt.Sprintf("\n❌ %s (%s):\n   %s\n   %s", tool.Name, tool.BinaryName, err, tool.Installation))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("missing required tools:%s", strings.Join(errors, ""))
	}

	return nil
}

// checkTool runs the version command to verify a tool is available and in PATH
func (tc *ToolChecker) checkTool(tool *Tool) error {
	cmd := exec.Command(tool.VersionCmd[0], tool.VersionCmd[1:]...)
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("%s not found in PATH or not executable", tool.BinaryName)
	}

	return nil
}

// IsAvailable runs a live check for whether a tool is installed and in PATH.
// Callers use this lazily, per call (per the "validate on invocation, not on
// startup" design), so it can't rely on ValidateAll() having run first.
func (tc *ToolChecker) IsAvailable(name string) bool {
	tool, ok := tc.tools[name]
	if !ok {
		return false
	}
	if err := tc.checkTool(tool); err != nil {
		return false
	}
	return true
}

// UnavailableMessage returns a user-facing "not available" message with
// install instructions for a tool, keyed by its BinaryName-derived key.
func (tc *ToolChecker) UnavailableMessage(name string) string {
	tool, ok := tc.tools[name]
	if !ok {
		return fmt.Sprintf("%s not available (no installation info known)", name)
	}
	return fmt.Sprintf("%s not available.\n\n%s", tool.Name, tool.Installation)
}
