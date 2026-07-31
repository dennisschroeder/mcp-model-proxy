package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// MCPServer handles MCP protocol communication with proper tool registration
type MCPServer struct {
	checker *ToolChecker
	model   string // selected model (chatgpt, gemini, antigravity)
}

// MCPMessage represents a message in the MCP protocol
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

// MCPTool represents an MCP tool definition
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// TextContent represents text content in MCP
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewMCPServer creates a new MCP server
func NewMCPServer(checker *ToolChecker) *MCPServer {
	model := os.Getenv("MCP_MODEL")
	if model == "" {
		model = "chatgpt"
	}

	return &MCPServer{
		checker: checker,
		model:   model,
	}
}

// Run starts the MCP server and listens for messages
func (s *MCPServer) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	log.Printf("MCP Server started with model: %s", s.model)

	for scanner.Scan() {
		line := scanner.Bytes()

		var msg MCPMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			s.sendError(encoder, -1, "Parse error", err)
			continue
		}

		switch msg.Method {
		case "initialize":
			s.handleInitialize(encoder, msg.ID)
		case "tools/list":
			s.handleListTools(encoder, msg.ID)
		case "tools/call":
			s.handleToolCall(encoder, msg.ID, msg.Params)
		default:
			s.sendError(encoder, msg.ID, "Unknown method", fmt.Errorf("method %s not implemented", msg.Method))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleInitialize responds to initialization with proper capabilities
func (s *MCPServer) handleInitialize(encoder *json.Encoder, id int) {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "mcp-model-proxy",
				"version": "0.2.0",
			},
		},
	}
	encoder.Encode(response)
}

// handleListTools returns available tools
func (s *MCPServer) handleListTools(encoder *json.Encoder, id int) {
	tools := []MCPTool{
		{
			Name:        "ask_model",
			Description: fmt.Sprintf("Send a message to the %s model and get a response", s.model),
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The message to send to the model",
					},
				},
				"required": []string{"message"},
			},
		},
	}

	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
	encoder.Encode(response)
}

// handleToolCall processes tool calls from Claude
func (s *MCPServer) handleToolCall(encoder *json.Encoder, id int, params json.RawMessage) {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		s.sendError(encoder, id, "Invalid params", err)
		return
	}

	switch callParams.Name {
	case "ask_model":
		s.handleAskModel(encoder, id, callParams.Arguments)
	default:
		s.sendError(encoder, id, "Unknown tool", fmt.Errorf("tool %s not found", callParams.Name))
	}
}

// handleAskModel processes model queries with lazy dependency checking
func (s *MCPServer) handleAskModel(encoder *json.Encoder, id int, args map[string]interface{}) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		s.sendError(encoder, id, "Invalid argument", fmt.Errorf("message required and must be a non-empty string"))
		return
	}

	// Lazy dependency check — only when model is actually invoked
	if err := s.checkDependency(s.model); err != nil {
		s.sendError(encoder, id, "Dependency error", err)
		return
	}

	// Call the model
	response, err := s.callModel(message)
	if err != nil {
		s.sendError(encoder, id, "Model call failed", err)
		return
	}

	// Success response with proper MCP format
	result := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []TextContent{
				{
					Type: "text",
					Text: response,
				},
			},
		},
	}
	encoder.Encode(result)
}

// checkDependency validates a specific model's dependencies
func (s *MCPServer) checkDependency(model string) error {
	switch strings.ToLower(model) {
	case "chatgpt":
		if !s.checker.IsAvailable("openai") {
			return fmt.Errorf("OpenAI CLI not available.\n\nInstall with:\n  pip install openai\n\nThen set your API key:\n  export OPENAI_API_KEY=sk-...")
		}
	case "gemini":
		if !s.checker.IsAvailable("gcloud") {
			return fmt.Errorf("Google Cloud CLI not available.\n\nInstall with:\n  brew install google-cloud-sdk\n\nThen authenticate:\n  gcloud auth application-default login")
		}
	case "antigravity":
		if !s.checker.IsAvailable("antigravity") {
			return fmt.Errorf("Antigravity CLI not available.\n\nInstall with:\n  brew install antigravity-cli\n\nThen authenticate:\n  antigravity-cli auth login")
		}
	default:
		return fmt.Errorf("unknown model: %s", model)
	}
	return nil
}

// callModel routes the message to the selected model provider
func (s *MCPServer) callModel(message string) (string, error) {
	switch strings.ToLower(s.model) {
	case "chatgpt":
		return s.callChatGPT(message)
	case "gemini":
		return s.callGemini(message)
	case "antigravity":
		return s.callAntigravity(message)
	default:
		return "", fmt.Errorf("unknown model: %s", s.model)
	}
}

// callChatGPT sends a message to OpenAI's ChatGPT via openai CLI
func (s *MCPServer) callChatGPT(message string) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`echo %q | openai api chat_completions.create -m gpt-4 -t 0.7`, message))

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
		"--model=gemini-pro",
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
	cmd := exec.Command("antigravity-cli", "ask", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Antigravity call failed: %w", err)
	}

	return string(output), nil
}

// sendError sends a properly formatted MCP error response
func (s *MCPServer) sendError(encoder *json.Encoder, id int, code string, err error) {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    code,
			"message": err.Error(),
		},
	}
	encoder.Encode(response)
}
