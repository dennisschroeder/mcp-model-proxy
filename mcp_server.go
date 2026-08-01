package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer wraps the SDK server with this proxy's model-routing state.
type MCPServer struct {
	checker *ToolChecker
	model   string // configured default route (chatgpt, gemini, antigravity, claude, codex)
	server  *mcp.Server
}

// Model names for the single-model direct routes, defined once so their
// call* functions and handleListModels/availableModels can't drift apart.
const (
	chatGPTModel = "gpt-4"
	geminiModel  = "gemini-pro"
	codexModel   = "gpt-5.6-sol" // codex's own current default, per `codex exec` with no --model flag
)

// claudeModels are the model aliases the claude CLI's --model flag accepts.
// claudeModels[0] is the default used when a call doesn't override the model.
var claudeModels = []string{"sonnet", "opus", "fable"}

// NewMCPServer creates a new MCP server and registers its tools. Tool
// input schemas are inferred by the SDK from the handler's input struct
// (jsonschema tags below), and input validation happens before the
// handler ever runs.
func NewMCPServer(checker *ToolChecker) *MCPServer {
	model := os.Getenv("MCP_MODEL")
	if model == "" {
		model = "chatgpt"
	}

	s := &MCPServer{checker: checker, model: model}
	s.server = mcp.NewServer(&mcp.Implementation{Name: "mcp-model-proxy", Version: "0.3.0"}, nil)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "ask_model",
		Description: fmt.Sprintf("Send a message to a model and get a response. Defaults to the configured %s route; pass model (see list_models) to use a specific model instead", s.model),
	}, s.handleAskModel)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_models",
		Description: "List concrete models grouped by actual provider, which route reaches each, and whether that route is currently available",
	}, s.handleListModels)

	return s
}

// Run starts the MCP server on stdio.
func (s *MCPServer) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// AskModelInput is ask_model's input; the SDK infers its JSON schema from
// these fields (required unless the json tag has omitempty).
type AskModelInput struct {
	Message  string `json:"message" jsonschema:"the message to send to the model"`
	Model    string `json:"model,omitempty" jsonschema:"optional: a specific model name from list_models (e.g. gemini-3.6-flash-high, claude-sonnet-4-6, gpt-4); determines routing automatically; omit to use the configured default route"`
	Provider string `json:"provider,omitempty" jsonschema:"optional cross-check: the provider (e.g. Google, OpenAI) the given model must belong to; requires model to also be set"`
}

// handleAskModel processes model queries with lazy dependency checking.
// If input.Model is set, it's resolved against availableModels() and
// routed automatically — overriding the server's configured default route
// for this call only. Otherwise falls back to the configured route.
//
// A non-nil returned error is a tool-level failure: the SDK packs it into
// CallToolResult.Content with IsError set, so the caller sees the message
// and can self-correct, rather than a raw MCP protocol error.
func (s *MCPServer) handleAskModel(ctx context.Context, req *mcp.CallToolRequest, input AskModelInput) (*mcp.CallToolResult, any, error) {
	if input.Message == "" {
		return nil, nil, fmt.Errorf("message required and must be a non-empty string")
	}

	if input.Provider != "" && input.Model == "" {
		return nil, nil, fmt.Errorf("provider requires model to also be set")
	}

	if input.Model != "" {
		return s.askModelOverride(input.Model, input.Provider, input.Message)
	}

	if !s.isKnownModel(s.model) {
		return nil, nil, fmt.Errorf("unknown model configured: %s", s.model)
	}

	// Lazy dependency check — only when model is actually invoked
	if err := s.checkDependency(s.model); err != nil {
		return nil, nil, err
	}

	response, err := s.callModel(input.Message)
	if err != nil {
		return nil, nil, err
	}

	return textResult(response), nil, nil
}

// resolveModel finds the entry in available named name. If provider is
// non-empty, it must match that entry's actual provider (routeProvider),
// else resolveModel reports the mismatch instead of silently ignoring it.
func resolveModel(available []routeModel, name, provider string) (*routeModel, error) {
	for _, m := range available {
		if m.name != name {
			continue
		}
		if provider != "" && !strings.EqualFold(routeProvider[m.route], provider) {
			return nil, fmt.Errorf("model %s belongs to provider %s, not %s", name, routeProvider[m.route], provider)
		}
		found := m
		return &found, nil
	}
	return nil, fmt.Errorf("unknown model: %s (call list_models to see available models)", name)
}

// askModelOverride routes a single ask_model call to a caller-chosen model,
// resolved against the same catalog list_models reports from.
func (s *MCPServer) askModelOverride(model, provider, message string) (*mcp.CallToolResult, any, error) {
	match, err := resolveModel(s.availableModels(), model, provider)
	if err != nil {
		return nil, nil, err
	}
	if !match.available {
		return nil, nil, errors.New(s.checker.UnavailableMessage(routes[match.route].tool))
	}

	var response string
	switch match.route {
	case "antigravity":
		response, err = s.callAntigravityWithModel(model, message)
	case "claude":
		response, err = s.callClaudeWithModel(model, message)
	default:
		response, err = routes[match.route].call(s, message)
	}
	if err != nil {
		return nil, nil, err
	}

	return textResult(response), nil, nil
}

// textResult wraps a plain-text response in an MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// ListModelsInput takes no arguments.
type ListModelsInput struct{}

// handleListModels reports, per provider, which concrete models this proxy
// can reach and via which route — proxying antigravity's own model list
// (`agy models`) rather than hardcoding it.
func (s *MCPServer) handleListModels(ctx context.Context, req *mcp.CallToolRequest, input ListModelsInput) (*mcp.CallToolResult, any, error) {
	return textResult(formatModelsByProvider(s.availableModels())), nil, nil
}

// routeModel is one concrete model reachable through a given route, and
// whether that route is currently available (its CLI tool installed).
type routeModel struct {
	name      string
	route     string
	available bool
}

// routeProvider maps each route to the company that makes it — antigravity
// is Google's CLI, so every model `agy models` returns is listed under
// Google, whatever the individual model names look like.
var routeProvider = map[string]string{
	"chatgpt":     "OpenAI",
	"gemini":      "Google",
	"antigravity": "Google",
	"claude":      "Anthropic",
	"codex":       "OpenAI",
}

// availableModels enumerates every concrete model this proxy can currently
// reach, across all routes. Shared by handleListModels and
// askModelOverride so both resolve models against the same catalog.
func (s *MCPServer) availableModels() []routeModel {
	var found []routeModel

	found = append(found, routeModel{
		name:      chatGPTModel,
		route:     "chatgpt",
		available: s.checker.IsAvailable(routes["chatgpt"].tool),
	})
	found = append(found, routeModel{
		name:      geminiModel,
		route:     "gemini",
		available: s.checker.IsAvailable(routes["gemini"].tool),
	})

	if s.checker.IsAvailable(routes["antigravity"].tool) {
		if out, err := s.listAntigravityModels(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				found = append(found, routeModel{name: line, route: "antigravity", available: true})
			}
		}
	}

	claudeAvailable := s.checker.IsAvailable(routes["claude"].tool)
	for _, name := range claudeModels {
		found = append(found, routeModel{name: name, route: "claude", available: claudeAvailable})
	}

	found = append(found, routeModel{
		name:      codexModel,
		route:     "codex",
		available: s.checker.IsAvailable(routes["codex"].tool),
	})

	return found
}

// formatModelsByProvider renders a grouped, sorted plain-text listing of
// models by provider — the text the list_models tool returns.
func formatModelsByProvider(found []routeModel) string {
	byProvider := map[string][]routeModel{}
	for _, m := range found {
		byProvider[routeProvider[m.route]] = append(byProvider[routeProvider[m.route]], m)
	}

	providers := make([]string, 0, len(byProvider))
	for p := range byProvider {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	var sb strings.Builder
	for _, p := range providers {
		sb.WriteString(p + "\n")
		for _, m := range byProvider[p] {
			status := "not available"
			if m.available {
				status = "available"
			}
			fmt.Fprintf(&sb, "  - %s (via %s, %s)\n", m.name, m.route, status)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// routeInfo pairs an MCP_MODEL route's required CLI tool key (checker.go
// looks up install instructions by this key) with its dispatch function.
// A route is a CLI tool this proxy talks to (chatgpt→openai, gemini→gcloud,
// antigravity→agy) — NOT itself a provider name (see routeProvider).
// This is the single source of truth for which routes exist — isKnownModel,
// checkDependency, and callModel all derive from it, so a route can't be
// "known" to one and unroutable in another.
type routeInfo struct {
	tool string
	call func(*MCPServer, string) (string, error)
}

var routes = map[string]routeInfo{
	"chatgpt":     {tool: "openai", call: (*MCPServer).callChatGPT},
	"gemini":      {tool: "gcloud", call: (*MCPServer).callGemini},
	"antigravity": {tool: "antigravity", call: (*MCPServer).callAntigravity},
	"claude":      {tool: "claude", call: (*MCPServer).callClaude},
	"codex":       {tool: "codex", call: (*MCPServer).callCodex},
}

// isKnownModel reports whether model is one of the supported routes
func (s *MCPServer) isKnownModel(model string) bool {
	_, ok := routes[strings.ToLower(model)]
	return ok
}

// checkDependency validates a specific route's dependencies
func (s *MCPServer) checkDependency(model string) error {
	m, ok := routes[strings.ToLower(model)]
	if !ok {
		return fmt.Errorf("unknown model: %s", model)
	}
	if !s.checker.IsAvailable(m.tool) {
		return errors.New(s.checker.UnavailableMessage(m.tool))
	}
	return nil
}

// callModel routes the message via the configured route
func (s *MCPServer) callModel(message string) (string, error) {
	m, ok := routes[strings.ToLower(s.model)]
	if !ok {
		return "", fmt.Errorf("unknown model: %s", s.model)
	}
	return m.call(s, message)
}

// callChatGPT sends a message to OpenAI's ChatGPT via openai CLI
func (s *MCPServer) callChatGPT(message string) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`echo %q | openai api chat_completions.create -m %s -t 0.7`, message, chatGPTModel))

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "OPENAI_API_KEY") || strings.Contains(outputStr, "authentication") {
			return "", fmt.Errorf("Authentication failed: OPENAI_API_KEY not set or invalid")
		}
		return "", fmt.Errorf("ChatGPT call failed: %w", err)
	}

	return string(output), nil
}

// callGemini sends a message to Google's Gemini via gcloud CLI
func (s *MCPServer) callGemini(message string) (string, error) {
	cmd := exec.Command("gcloud", "beta", "ai", "models", "predict",
		"--model="+geminiModel,
		"--region=us-central1",
		fmt.Sprintf("--input=%s", message),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Gemini call failed: %w", err)
	}

	return string(output), nil
}

// callAntigravity sends a message to Antigravity CLI (test provider)
func (s *MCPServer) callAntigravity(message string) (string, error) {
	cmd := exec.Command("agy", "--print", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Antigravity call failed: %w", err)
	}

	return string(output), nil
}

// callAntigravityWithModel sends a message to a specific model via the
// Antigravity CLI, for ask_model calls that override the default route.
func (s *MCPServer) callAntigravityWithModel(model, message string) (string, error) {
	cmd := exec.Command("agy", "--model", model, "--print", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Antigravity call failed: %w", err)
	}

	return string(output), nil
}

// listAntigravityModels proxies `agy models` to list the concrete models
// available through the Antigravity CLI (e.g. underlying Gemini/Claude/GPT variants).
func (s *MCPServer) listAntigravityModels() (string, error) {
	cmd := exec.Command("agy", "models")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("agy models failed: %w", err)
	}

	return string(output), nil
}

// callClaude sends a message to the Claude CLI using its own default model
func (s *MCPServer) callClaude(message string) (string, error) {
	cmd := exec.Command("claude", "-p", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Claude call failed: %w", err)
	}

	return string(output), nil
}

// callClaudeWithModel sends a message to a specific Claude model/alias
// (see claudeModels), for ask_model calls that override the default route.
func (s *MCPServer) callClaudeWithModel(model, message string) (string, error) {
	cmd := exec.Command("claude", "--model", model, "-p", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Claude call failed: %w", err)
	}

	return string(output), nil
}

// callCodex sends a message to OpenAI's Codex CLI. --skip-git-repo-check
// since this proxy's cwd is wherever Claude Desktop launched it from, not
// necessarily a git repo Codex would otherwise refuse to run outside of.
func (s *MCPServer) callCodex(message string) (string, error) {
	cmd := exec.Command("codex", "exec", "--skip-git-repo-check", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Codex call failed: %w", err)
	}

	return string(output), nil
}
