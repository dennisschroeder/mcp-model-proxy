# Claude Code / Desktop Integration

## Overview

Once the MCP server is running, it exposes a standardized MCP interface that Claude Code can connect to.

## Setup Steps

### 1. Build the server

```bash
cd ~/Developer/mcp-model-proxy
go build -o mcp-model-proxy .
```

### 2. Install CLI dependencies

**For ChatGPT:**
```bash
pip install openai
export OPENAI_API_KEY=sk-...
```

**For Gemini:**
```bash
brew install google-cloud-sdk
gcloud auth application-default login
```

### 3. Run the server

```bash
# Start with default model (chatgpt)
./mcp-model-proxy

# Or select a model
MCP_MODEL=gemini ./mcp-model-proxy
```

The server will:
1. Check that all required CLI tools are installed
2. Validate they're in PATH and executable
3. Start listening on stdin/stdout for MCP messages
4. Route incoming messages to the selected provider

If a tool is missing, it will fail fast with installation instructions.

## Claude Code Configuration

To connect this MCP server to Claude Code:

1. Add to `.claude/launch.json`:
```json
{
  "version": "0.0.1",
  "configurations": [
    {
      "name": "mcp-model-proxy",
      "runtimeExecutable": "~/Developer/mcp-model-proxy/mcp-model-proxy",
      "runtimeArgs": [],
      "port": null,
      "env": {
        "MCP_MODEL": "chatgpt"
      }
    }
  ]
}
```

2. Or launch manually:
```bash
MCP_MODEL=gemini ~/Developer/mcp-model-proxy/mcp-model-proxy
```

## Usage

Once connected via MCP:

- Claude Code sends messages to the proxy
- The proxy routes to the selected provider (ChatGPT or Gemini)
- The provider's response is returned to Claude Code

## Error Handling

**Hard-fail by design:**
- If a model request fails, the error is returned to Claude Code
- Claude Code can then choose to retry with a different model
- The proxy stays simple and doesn't try to be clever about fallbacks

**Example flow:**
1. User selects ChatGPT as the model
2. Message fails (e.g., rate limit)
3. Proxy returns error to Claude Code
4. User manually switches to Gemini via `MCP_MODEL=gemini`
5. Retry with new provider

This keeps the proxy transparent and puts control with the user.
