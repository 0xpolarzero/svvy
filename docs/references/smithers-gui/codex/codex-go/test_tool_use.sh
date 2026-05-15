#!/bin/bash

# Simple test to verify tool use is working
# This will send a message asking Claude to use a tool

export ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}"
export MODEL="${MODEL:-claude-3-5-sonnet-20241022}"

if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Error: ANTHROPIC_API_KEY environment variable is not set"
    exit 1
fi

echo "Testing tool use with message: 'What files are in the current directory?'"
echo "================================================"
./codex -m "What files are in the current directory? Use the list_dir tool."
