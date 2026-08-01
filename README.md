# MCP Model Proxy

A Go MCP server that lets any MCP-compatible client — Claude Code, Claude Desktop, opencode, or any other MCP-aware CLI/harness — delegate a prompt to another LLM provider — OpenAI, Google, or Anthropic — by proxying through that provider's own CLI tool.

It's a dumb proxy by design: it doesn't inspect or rewrite the message you send it, it just routes it to the CLI tool for the model you asked for and returns the raw output.

## Providers

`list_models` reports, live, which models this proxy can currently reach — grouped by provider, with each model's availability determined by whether its underlying CLI tool is installed and authenticated:

- **Google** — Gemini models (via `gcloud`), plus whatever models the Antigravity CLI (`agy`) itself reports (`agy models`)
- **Anthropic** — Claude models: `sonnet`, `opus`, `fable` (via the `claude` CLI)
- **OpenAI** — GPT models (via the `openai` CLI), plus Codex's own model (via the `codex` CLI)

## Prerequisites

The MCP server itself has no dependencies beyond Go to build it. Each provider's models, however, require their underlying CLI tool to be **installed and authenticated, and on the same `PATH` the MCP server process runs with.** This last part is a common gotcha for GUI-launched apps like Claude Desktop, whose PATH can differ from your login shell's — if a tool works in your terminal but `list_models` still reports it unavailable, check what PATH the Claude Desktop process actually sees.

You don't need all three installed — only install the ones for the providers you want to use. `list_models` will report the rest as unavailable with install instructions.

| Tool | Provider route | Install |
|---|---|---|
| `agy` (Google Antigravity CLI) | Google (Gemini via Antigravity) | [antigravity.google/docs/cli/install](https://antigravity.google/docs/cli/install) — note the binary installs as `agy`, not `antigravity-cli` |
| `claude` (Claude Code CLI) | Anthropic (Claude) | [code.claude.com/docs](https://code.claude.com/docs/en/overview), or `npm install -g @anthropic-ai/claude-code` |
| `codex` (OpenAI Codex CLI) | OpenAI (Codex) | [developers.openai.com/codex/cli](https://developers.openai.com/codex/cli), or `brew install --cask codex`, then `codex login` |

## Install

**Prebuilt binary** — download the archive for your platform from the [latest release](https://github.com/dennisschroeder/mcp-model-proxy/releases/latest) and extract the `mcp-model-proxy` binary.

**From source:**
```bash
go install github.com/dennisschroeder/mcp-model-proxy@latest
```
or, from a clone:
```bash
git clone https://github.com/dennisschroeder/mcp-model-proxy.git
cd mcp-model-proxy
go build -o mcp-model-proxy .
```

## Configuring Claude Desktop / Claude Code

Add an entry to Claude Desktop's `claude_desktop_config.json` (or Claude Code's `.claude/launch.json`):

```json
{
  "mcpServers": {
    "mcp-model-proxy": {
      "command": "/absolute/path/to/mcp-model-proxy",
      "env": {
        "MCP_MODEL": "chatgpt"
      }
    }
  }
}
```

`MCP_MODEL` sets the default route used when `ask_model` is called without an explicit `model` — one of `chatgpt`, `gemini`, `antigravity`, `claude`, `codex`. It's optional; it defaults to `chatgpt`. Any call can override it per-request regardless (see below), so this only matters if you want a different default.

Restart Claude Desktop/Code after editing the config.

## MCP Tools

### `list_models`
No arguments. Returns the live catalog of models grouped by provider, with each model's availability (installed + reachable) and which CLI tool backs it.

### `ask_model`
```json
{
  "message": "What is the capital of France?",
  "model": "sonnet",
  "provider": "Anthropic"
}
```
- `message` (required) — the prompt to send.
- `model` (optional) — a specific model name from `list_models`. Omit to use the server's configured default route (`MCP_MODEL`).
- `provider` (optional) — a cross-check: if set, `model` must belong to this provider, or the call fails with an explicit mismatch error instead of silently routing somewhere unexpected. Requires `model` to also be set.

Dependency checks are lazy — a missing/unauthenticated CLI tool only produces an error at call time (with install instructions), not at server startup.

## Architecture

- **`main.go`** — entry point; wires up the tool checker and MCP server, then serves on stdio.
- **`checker.go`** — `ToolChecker`: live availability checks for each CLI tool, plus user-facing install instructions when one's missing.
- **`mcp_server.go`** — registers `ask_model`/`list_models` via the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk), resolves model/provider routing, and dispatches to each CLI tool.

Built on `github.com/modelcontextprotocol/go-sdk/mcp`, which handles the MCP protocol handshake, JSON schema inference, and input validation.

## Development

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .   # should print nothing
```

## License

[MIT](LICENSE)
