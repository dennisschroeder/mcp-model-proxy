# MCP Model Proxy — MVP Prototype Summary

## What We Built

A Go-based MCP server that routes messages to multiple LLM providers via their CLI tools.

### Core Features

✓ **Hard-fail dependency checking** — On startup, validates all required CLI tools (gcloud, openai) are installed and in PATH. If missing, suggests installation with actionable instructions.

✓ **Model selection** — Choose model via `MCP_MODEL` environment variable (chatgpt, gemini). Defaults to chatgpt.

✓ **MCP protocol** — Implements MCP 2024-11-05 protocol for communication with Claude Code.

✓ **Provider routing** — Routes messages to ChatGPT via `openai` CLI or Gemini via `gcloud` CLI.

✓ **Hard-fail error handling** — Errors returned to Claude Code; Claude Code decides what to do (retry, switch provider, etc.).

### Architecture

- **main.go** — Entry point, runs dependency check, starts MCP server
- **checker.go** — Tool validator; checks availability and in-PATH status
- **mcp_server.go** — MCP protocol handler; routes to providers

### Files

```
mcp-model-proxy/
├── main.go                    # Entry point
├── checker.go                 # CLI tool validation (87 lines)
├── mcp_server.go              # MCP protocol + provider routing (219 lines)
├── go.mod                     # Go module definition
├── README.md                  # Quick start guide
├── claude-integration.md      # How to connect to Claude Code
├── test-setup.sh              # Dependency check test script
└── PROTOTYPE-SUMMARY.md       # This file
```

### How It Works

1. **Startup**
   - Parse `MCP_MODEL` env var (default: chatgpt)
   - Create ToolChecker, validate all CLI tools
   - If any tool missing → fail fast with install instructions
   - Start MCP server on stdin/stdout

2. **Message Flow**
   - Receive MCP message on stdin
   - Route to selected provider's CLI tool
   - Return response via stdout

3. **Error Handling**
   - All errors returned to Claude Code
   - Claude Code chooses what to do (retry, switch model, etc.)
   - Proxy stays transparent

## What's NOT in the MVP (by design)

❌ Config file — just use env vars for now
❌ Provider fallback chains — hard-fail instead
❌ Cost tracking — outside MVP scope
❌ Model-specific tuning — using defaults
❌ Caching — stateless server

## Testing

Run the tool checker:
```bash
./test-setup.sh
```

This will:
- Check which CLI tools are installed
- Verify environment variables (OPENAI_API_KEY, etc.)
- Run the server and show validation output

## Integration with Claude Code

See `claude-integration.md` for:
- Setup steps for each provider
- How to configure `.claude/launch.json`
- Usage patterns

## Next Iteration (Future)

If you want to extend this:

1. **Configuration file** — `.mcp-proxy/config.toml` for provider mappings
2. **Graceful fallbacks** — Configurable chain: try X, if fails try Y
3. **Provider detection** — Auto-discover available providers on startup
4. **Cost tracking** — Log which provider was used and cost
5. **Response parsing** — Extract actual LLM response from CLI output (currently raw CLI output)
6. **Retry logic** — Built-in retries with backoff
7. **Logging/tracing** — Structured logging for debugging

## Key Design Decisions

**Hard-fail on missing tools**: Better to fail loudly at startup than mysteriously at message time.

**Delegate auth to CLI tools**: Reuses existing auth (gcloud login, OPENAI_API_KEY). No key management in proxy.

**No fallback chains in proxy**: Puts decision with Claude Code. Simpler architecture, users have control.

**No response transformation**: Raw CLI output returned. Future iteration can add parsing if needed.

## Known Limitations

- OpenAI CLI invocation may not work exactly as written (need to test against real openai CLI API)
- Gemini invocation via gcloud may need adjustment for actual Vertex AI API
- No support for streaming responses yet
- No support for vision/multimodal yet
- Model parameters hardcoded (temperature, model name, etc.)

These should be addressed when integrating with real CLI tools.
