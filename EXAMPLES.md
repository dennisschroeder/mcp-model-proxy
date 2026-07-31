# MCP Model Proxy — Examples

## Startup Output (Missing Dependencies)

When you run the server without required tools installed:

```bash
$ ./mcp-model-proxy
2026/07/31 19:43:50 Dependency check failed:
Missing required tools:
❌ Google Cloud CLI (gcloud):
   gcloud not found in PATH or not executable
   Install gcloud CLI:
  macOS: brew install google-cloud-sdk
  Linux: https://cloud.google.com/sdk/docs/install
  Windows: https://cloud.google.com/sdk/docs/install
Then run: gcloud auth application-default login
❌ OpenAI CLI (openai):
   openai not found in PATH or not executable
   Install OpenAI CLI:
  macOS/Linux: pip install openai
  Windows: pip install openai
Then set: export OPENAI_API_KEY=sk-...
```

**Action**: Install the suggested tools, then restart.

## Startup Output (All Good)

After installing dependencies:

```bash
$ MCP_MODEL=chatgpt ./mcp-model-proxy
2026/07/31 19:44:12 ✓ All dependencies validated
2026/07/31 19:44:12 Starting MCP server...
2026/07/31 19:44:12 MCP Server started with model: chatgpt
```

The server is now ready to receive MCP messages on stdin.

## MCP Protocol Example

### Initialize Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {}
}
```

### Initialize Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "mcp-model-proxy",
      "version": "0.1.0"
    }
  }
}
```

### Message Call Request

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "message": "What is the capital of France?"
    }
  }
}
```

### Message Call Response (Success)

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "The capital of France is Paris."
      }
    ]
  }
}
```

### Error Response Example

If ChatGPT fails (e.g., rate limit):

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": -1,
    "message": "Model error",
    "data": {
      "detail": "ChatGPT call failed: exit status 1\nOutput: Rate limit exceeded"
    }
  }
}
```

**Claude Code's action**: Switch to Gemini by restarting with `MCP_MODEL=gemini`, or retry later.

## Environment Variables

```bash
# Select which model to use
export MCP_MODEL=chatgpt  # default
export MCP_MODEL=gemini

# OpenAI authentication
export OPENAI_API_KEY=sk-...

# Google Cloud (handled by gcloud CLI)
# Run: gcloud auth application-default login
```

## Quick Start Script

```bash
#!/bin/bash
# setup-proxy.sh

# 1. Build
cd ~/Developer/mcp-model-proxy
go build -o mcp-model-proxy .

# 2. Install OpenAI CLI
pip install openai
export OPENAI_API_KEY=sk-...

# 3. Test with ChatGPT
./mcp-model-proxy &
PID=$!

# 4. Send a test message (simulating Claude Code)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./mcp-model-proxy

kill $PID
```

## Switching Providers

```bash
# Start with ChatGPT
MCP_MODEL=chatgpt ./mcp-model-proxy &

# Later, restart with Gemini if ChatGPT fails
pkill mcp-model-proxy
MCP_MODEL=gemini ./mcp-model-proxy &
```

In a full integration with Claude Code, you'd configure this via `.claude/launch.json` or an environment file.
