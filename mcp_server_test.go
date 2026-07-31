package main

import (
	"strings"
	"testing"
)

func TestResolveModel(t *testing.T) {
	available := []routeModel{
		{name: "gpt-4", route: "chatgpt", available: false},
		{name: "gemini-3.6-flash-high", route: "antigravity", available: true},
		{name: "sonnet", route: "claude", available: true},
	}

	t.Run("finds a known model with no provider check", func(t *testing.T) {
		m, err := resolveModel(available, "sonnet", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.route != "claude" {
			t.Errorf("route = %q, want claude", m.route)
		}
	})

	t.Run("accepts a matching provider (case-insensitive)", func(t *testing.T) {
		m, err := resolveModel(available, "sonnet", "anthropic")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.name != "sonnet" {
			t.Errorf("name = %q, want sonnet", m.name)
		}
	})

	t.Run("rejects a mismatched provider", func(t *testing.T) {
		_, err := resolveModel(available, "sonnet", "OpenAI")
		if err == nil {
			t.Fatal("expected error for provider mismatch, got nil")
		}
	})

	t.Run("rejects an unknown model", func(t *testing.T) {
		_, err := resolveModel(available, "not-a-real-model", "")
		if err == nil {
			t.Fatal("expected error for unknown model, got nil")
		}
	})

	t.Run("reports availability of the resolved model", func(t *testing.T) {
		m, err := resolveModel(available, "gpt-4", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.available {
			t.Error("gpt-4 entry should be unavailable per the fixture")
		}
	})
}

// modelsUnderProvider returns the "- name (via route, status)" lines that
// formatModelsByProvider printed directly under the given provider header.
func modelsUnderProvider(out, provider string) []string {
	var lines []string
	inSection := false
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "  ") {
			inSection = line == provider
			continue
		}
		if inSection {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func TestFormatModelsByProvider(t *testing.T) {
	found := []routeModel{
		{name: "gpt-4", route: "chatgpt", available: false},
		{name: "gemini-pro", route: "gemini", available: false},
		{name: "gemini-3.6-flash-high", route: "antigravity", available: true},
		{name: "claude-sonnet-4-6", route: "antigravity", available: true},
		{name: "sonnet", route: "claude", available: true},
	}

	out := formatModelsByProvider(found)

	t.Run("groups antigravity's models under Google, not by model name", func(t *testing.T) {
		// claude-sonnet-4-6 comes back from `agy models`, but antigravity is
		// Google's CLI, so it must land under Google, never under Anthropic.
		google := modelsUnderProvider(out, "Google")
		if !containsPrefix(google, "- claude-sonnet-4-6") {
			t.Errorf("expected claude-sonnet-4-6 under Google, got Google section: %v\nfull output:\n%s", google, out)
		}

		anthropic := modelsUnderProvider(out, "Anthropic")
		if containsPrefix(anthropic, "- claude-sonnet-4-6") {
			t.Errorf("claude-sonnet-4-6 must not appear under Anthropic, got: %v", anthropic)
		}
	})

	t.Run("marks unavailable routes as not available", func(t *testing.T) {
		openAI := modelsUnderProvider(out, "OpenAI")
		if !containsPrefix(openAI, "- gpt-4 (via chatgpt, not available)") {
			t.Errorf("expected gpt-4 marked not available, got: %v", openAI)
		}
	})

	t.Run("marks available routes as available", func(t *testing.T) {
		anthropic := modelsUnderProvider(out, "Anthropic")
		if !containsPrefix(anthropic, "- sonnet (via claude, available)") {
			t.Errorf("expected sonnet marked available, got: %v", anthropic)
		}
	})

	t.Run("provider sections are sorted alphabetically", func(t *testing.T) {
		var headers []string
		for _, line := range strings.Split(out, "\n") {
			if !strings.HasPrefix(line, "  ") {
				headers = append(headers, line)
			}
		}
		want := []string{"Anthropic", "Google", "OpenAI"}
		if len(headers) != len(want) {
			t.Fatalf("headers = %v, want %v", headers, want)
		}
		for i := range want {
			if headers[i] != want[i] {
				t.Errorf("headers = %v, want %v", headers, want)
				break
			}
		}
	})
}

func containsPrefix(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

func TestIsKnownModel(t *testing.T) {
	s := &MCPServer{}

	cases := map[string]bool{
		"chatgpt":     true,
		"gemini":      true,
		"antigravity": true,
		"claude":      true,
		"codex":       true,
		"CHATGPT":     true, // case-insensitive
		"not-a-route": false,
		"":            false,
	}

	for model, want := range cases {
		if got := s.isKnownModel(model); got != want {
			t.Errorf("isKnownModel(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestRouteProviderCoversEveryRoute(t *testing.T) {
	for route := range routes {
		if _, ok := routeProvider[route]; !ok {
			t.Errorf("route %q has no entry in routeProvider", route)
		}
	}
}
