# MCP Model Proxy

A Go-based MCP server that routes messages to multiple LLM providers (ChatGPT, Gemini) via their CLI tools.

## Features

- **Hard-fail validation** — checks all required CLI tools on startup, suggests installation if missing
- **Model selection** — choose model via `MCP_MODEL` environment variable or configuration
- **Extensible provider architecture** — easy to add new providers
- **Minimal dependencies** — relies on pre-installed CLI tools, no extra API integrations

## Prerequisites

### ChatGPT Support
```bash
pip install openai
export OPENAI_API_KEY=sk-...  # Your OpenAI API key
```

### Gemini Support
```bash
brew install google-cloud-sdk  # or your platform's install method
gcloud auth application-default login
```

## Building

```bash
go build -o mcp-model-proxy .
```

## Running

```bash
# Use default model (chatgpt)
./mcp-model-proxy

# Use a specific model
MCP_MODEL=gemini ./mcp-model-proxy
```

## Architecture

### Tool Checker
Validates all dependencies on startup. If a required tool is missing or not in PATH, it fails fast with actionable installation instructions.

### MCP Server
Listens on stdin/stdout for MCP protocol messages. Routes tool calls to the selected provider.

### Model Providers
- **ChatGPT** — via `openai` CLI
- **Gemini** — via `gcloud` CLI

## MCP Protocol

The server implements the MCP 2024-11-05 protocol with:
- `initialize` — handshake
- `tools/call` — pass messages to the selected model

## Next Steps (Not in MVP)

- [ ] Configuration file for provider mappings
- [ ] Provider-specific parameter tuning
- [ ] Caching/response logging
- [ ] Cost tracking across providers
- [ ] Graceful degradation patterns

## Development Notes

This is an MVP prototype. Error handling is hard-fail by design (let Claude Code choose the provider, not the proxy).
