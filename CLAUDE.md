# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...              # build
go vet ./...                # vet
go test ./...                # test (all)
go test -run TestResolveModel ./...   # single test
gofmt -l .                   # formatting check; must print nothing
```

Local release validation (no git remote required, unlike `goreleaser check`/`release`):

```bash
goreleaser build --snapshot --clean
```

Manual protocol smoke test (the MCP SDK requires the full handshake before it will answer `tools/list`/`tools/call` — a bare `tools/list` request with no prior `initialize` + `notifications/initialized` gets ignored):

```bash
(cat <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF
sleep 1) | ./mcp-model-proxy
```
(the trailing `sleep` matters — if stdin closes immediately after the last line, the SDK's stdio transport sees EOF and exits before writing the response)

## Architecture

Three files, single package: `main.go` (wiring) → `checker.go` (`ToolChecker`) → `mcp_server.go` (MCP tools + routing). Built on `github.com/modelcontextprotocol/go-sdk/mcp`, which owns the protocol handshake, JSON schema inference (from the `AskModelInput`/`ListModelsInput` struct tags), and input validation — this project does not hand-roll any JSON-RPC.

**Route vs. provider — the one distinction that matters most in this codebase.** A *route* (`chatgpt`, `gemini`, `antigravity`, `claude`, `codex` — keys in the `routes` map) is a CLI tool this proxy shells out to. A *provider* (`OpenAI`, `Google`, `Anthropic` — values in `routeProvider`) is the company that actually serves the model. These are not 1:1: `antigravity` (the `agy` binary) and `gemini` (`gcloud`) are both routes to Google; `chatgpt` (`openai`) and `codex` are both routes to OpenAI. User-facing output (`list_models`, the `provider` cross-check arg on `ask_model`) is always in terms of providers; internal dispatch (`routes` map, `checker.go` tool keys) is always in terms of routes. When adding a new CLI tool, it needs an entry in both `routes` (dispatch) and `routeProvider` (which company it actually belongs to) — `TestRouteProviderCoversEveryRoute` enforces this pairing never drifts.

**Single resolution path.** Both a per-call override (`ask_model`'s `model`/`provider` args) and the configured default (`DEFAULT_PROVIDER_MODEL` env var, parsed by `parseDefaultProviderModel` into the same `(model, provider)` shape) are resolved through the same `resolveModel` → `askModelOverride` path in `handleAskModel`. There is no separate "default route" dispatch — if you're tempted to add one when extending this, don't; route the default through `askModelOverride` like everything else so a misconfigured default fails the same way (a tool-level error, not a startup crash) as a bad per-call override.

**Lazy dependency validation.** Nothing is checked at startup (`main.go` starts serving immediately). `ToolChecker.IsAvailable` runs the tool's version command fresh on every call — there is no cached/eager validation path. This means `checker.go` and `mcp_server.go` re-check tool availability on literally every `ask_model`/`list_models` call; that's intentional, not an oversight.

**`agy models` is proxied live, not hardcoded.** `availableModels()` shells out to `agy models` to enumerate whatever concrete models the Antigravity CLI currently reports, rather than maintaining a static list — those all get attributed to the Google provider regardless of what the model names themselves look like (e.g. `agy` can report a model literally named `claude-sonnet-4-6`, and it still must land under Google, not Anthropic — see the test `groups antigravity's models under Google, not by model name` in `mcp_server_test.go`).

**This is a dumb proxy by design — no message-content-aware logic.** The server must not inspect, rewrite, or special-case the `message` string based on its content (e.g. detecting/normalizing URLs). That kind of logic belongs in a client-side skill/prompt layer, not here — this was an explicit design decision after a URL-normalization change was tried and reverted. If a feature request implies branching on what's *inside* a message, it's out of scope for this repo.

## Known environment gotchas (from real debugging, not speculation)

- The Homebrew cask `antigravity-cli` installs a binary named `agy`, not `antigravity-cli` — `checker.go`'s install instructions call this out explicitly; don't "fix" the binary name back to match the cask name.
- GUI-launched MCP clients (Claude Desktop) can see a different `PATH` than a login shell. A tool that works fine in a terminal can still be reported unavailable by `list_models` if Claude Desktop's process doesn't have it on `PATH`. This is the most common false-alarm bug report to expect.
- Binaries are built with `-trimpath -ldflags="-s -w"` (see `.goreleaser.yml`) specifically so local development machine paths (e.g. Go module cache paths) don't end up embedded in shipped release binaries. The module's own import path (`github.com/dennisschroeder/mcp-model-proxy`) still appears in the binary — that's expected and fine, it's public information, not a leak.
- `goreleaser check` and `goreleaser release` fail locally with "no remote configured to list refs from" if there's no `git remote` set — this is expected outside CI; use `goreleaser build --snapshot --clean` (see Commands above) to validate the build config without needing a remote.
