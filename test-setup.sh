#!/bin/bash
# Quick setup test for mcp-model-proxy dependencies

set -e

echo "=== MCP Model Proxy Dependency Check ==="
echo ""

# Check if tools are installed
echo "Checking for required tools..."
echo ""

# Check gcloud
if command -v gcloud &> /dev/null; then
    echo "✓ gcloud found: $(gcloud version --format='value(gke_backup/version)' 2>/dev/null || echo 'installed')"
else
    echo "✗ gcloud not found"
    echo "  Install: brew install google-cloud-sdk"
fi
echo ""

# Check openai
if command -v openai &> /dev/null; then
    echo "✓ openai found: $(openai --version 2>/dev/null || echo 'installed')"
    if [ -n "$OPENAI_API_KEY" ]; then
        echo "  ✓ OPENAI_API_KEY is set"
    else
        echo "  ✗ OPENAI_API_KEY not set"
        echo "    Set: export OPENAI_API_KEY=sk-..."
    fi
else
    echo "✗ openai not found"
    echo "  Install: pip install openai"
fi
echo ""

echo "=== Running mcp-model-proxy ==="
echo ""

# Try to run the server (will fail if dependencies are missing)
cd "$(dirname "$0")"
go build -o mcp-model-proxy . 2>&1 || exit 1

# Run and show the error output (expected to fail on missing tools)
./mcp-model-proxy || true
