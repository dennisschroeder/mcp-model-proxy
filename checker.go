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
	Name        string
	BinaryName  string
	VersionCmd  []string // command to check version/availability
	Installation string  // installation instructions
	IsAvailable bool
}

// NewToolChecker creates a new tool checker
func NewToolChecker() *ToolChecker {
	return &ToolChecker{
		tools: map[string]*Tool{
			"gcloud": {
				Name:       "Google Cloud CLI",
				BinaryName: "gcloud",
				VersionCmd: []string{"gcloud", "version"},
				Installation: `Install gcloud CLI:
  macOS: brew install google-cloud-sdk
  Linux: https://cloud.google.com/sdk/docs/install
  Windows: https://cloud.google.com/sdk/docs/install
Then run: gcloud auth application-default login`,
			},
			"openai": {
				Name:       "OpenAI CLI",
				BinaryName: "openai",
				VersionCmd: []string{"openai", "--version"},
				Installation: `Install OpenAI CLI:
  macOS/Linux: pip install openai
  Windows: pip install openai
Then set: export OPENAI_API_KEY=sk-...`,
			},
		},
	}
}

// ValidateAll checks all tools and returns aggregated errors
func (tc *ToolChecker) ValidateAll() error {
	var errors []string

	for key, tool := range tc.tools {
		if err := tc.checkTool(tool); err != nil {
			errors = append(errors, fmt.Sprintf("\n❌ %s (%s):\n   %s\n   %s", tool.Name, tool.BinaryName, err, tool.Installation))
		} else {
			tc.tools[key].IsAvailable = true
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("Missing required tools:%s", strings.Join(errors, ""))
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

// IsAvailable checks if a tool is available
func (tc *ToolChecker) IsAvailable(name string) bool {
	if tool, ok := tc.tools[name]; ok {
		return tool.IsAvailable
	}
	return false
}
