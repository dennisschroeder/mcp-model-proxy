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

// MCPServer handles MCP protocol communication
type MCPServer struct {
	checker *ToolChecker
	model   string // selected model (chatgpt, gemini)
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

// TextContent represents text content in MCP
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewMCPServer creates a new MCP server
func NewMCPServer(checker *ToolChecker) *MCPServer {
	// Default to chatgpt, can be overridden by environment or parameters
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

// handleInitialize responds to initialization
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
				"version": "0.1.0",
			},
		},
	}
	encoder.Encode(response)
}

// handleToolCall processes tool calls from Claude
func (s *MCPServer) handleToolCall(encoder *json.Encoder, id int, params json.RawMessage) {
	var callParams struct {
		Name      string `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		s.sendError(encoder, id, "Invalid params", err)
		return
	}

	// For now, treat all tool calls as "send message" requests
	userMessage, ok := callParams.Arguments["message"].(string)
	if !ok {
		s.sendError(encoder, id, "Missing message", fmt.Errorf("message argument required"))
		return
	}

	response, err := s.callModel(userMessage)
	if err != nil {
		s.sendError(encoder, id, "Model error", err)
		return
	}

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

// callModel routes the message to the selected model provider
func (s *MCPServer) callModel(message string) (string, error) {
	switch strings.ToLower(s.model) {
	case "chatgpt":
		return s.callChatGPT(message)
	case "gemini":
		return s.callGemini(message)
	default:
		return "", fmt.Errorf("unknown model: %s", s.model)
	}
}

// callChatGPT sends a message to OpenAI's ChatGPT via openai CLI
func (s *MCPServer) callChatGPT(message string) (string, error) {
	if !s.checker.IsAvailable("openai") {
		return "", fmt.Errorf("OpenAI CLI not available. Install with: pip install openai")
	}

	// Using openai CLI with the api command
	// Format: echo "message" | openai api chat_completions.create -m gpt-4 -t 0.7
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`echo %q | openai api chat_completions.create -m gpt-4 -t 0.7`, message))

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's an auth error
		outputStr := string(output)
		if strings.Contains(outputStr, "OPENAI_API_KEY") || strings.Contains(outputStr, "authentication") {
			return "", fmt.Errorf("OpenAI authentication failed. Set: export OPENAI_API_KEY=sk-...")
		}
		return "", fmt.Errorf("ChatGPT call failed: %w\nOutput: %s", err, outputStr)
	}

	return string(output), nil
}

// callGemini sends a message to Google's Gemini via gcloud CLI
func (s *MCPServer) callGemini(message string) (string, error) {
	if !s.checker.IsAvailable("gcloud") {
		return "", fmt.Errorf("gcloud CLI not available. Install with: brew install google-cloud-sdk")
	}

	// Using gcloud's Vertex AI API for Gemini
	// Format: gcloud beta ai models predict --model=gemini-pro --region=us-central1 --input="message"
	cmd := exec.Command("gcloud", "beta", "ai", "models", "predict",
		"--model=gemini-pro",
		"--region=us-central1",
		fmt.Sprintf("--input=%s", message),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's an auth error
		outputStr := string(output)
		if strings.Contains(outputStr, "authentication") || strings.Contains(outputStr, "credentials") {
			return "", fmt.Errorf("gcloud authentication failed. Run: gcloud auth application-default login")
		}
		return "", fmt.Errorf("Gemini call failed: %w\nOutput: %s", err, outputStr)
	}

	return string(output), nil
}

// sendError sends an error response
func (s *MCPServer) sendError(encoder *json.Encoder, id int, message string, err error) {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -1,
			"message": message,
			"data": map[string]interface{}{
				"detail": err.Error(),
			},
		},
	}
	encoder.Encode(response)
}
