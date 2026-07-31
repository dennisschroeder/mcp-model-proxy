#!/bin/bash
# Test MCP model proxy with a sample message

set -e

echo "=== Testing MCP Model Proxy ==="
echo ""
echo "Starting server with model: antigravity"
echo ""

# Start the server in the background
timeout 5 ./mcp-model-proxy <<EOF 2>&1 || true
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_message","arguments":{"message":"Hello from MCP test!"}}}
EOF

echo ""
echo "=== Test Complete ==="
echo ""
echo "Expected output:"
echo "1. Dependency check fails (antigravity-cli not installed)"
echo "2. Server provides installation instructions"
echo ""
echo "When dependencies are met, the server will:"
echo "1. Initialize successfully"
echo "2. Route messages to the selected provider"
