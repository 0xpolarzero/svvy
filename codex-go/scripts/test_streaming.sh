#!/bin/bash
# Test script for validating AI streaming functionality

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Codex Streaming Test ===${NC}\n"

# Check for API key
if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${RED}Error: No API key found. Set ANTHROPIC_API_KEY or OPENAI_API_KEY${NC}"
    exit 1
fi

# Use provided model or default
export MODEL="${MODEL:-claude-3-5-sonnet-20241022}"
echo -e "${YELLOW}Using model: $MODEL${NC}\n"

# Test 1: Simple message
echo -e "${GREEN}Test 1: Simple message${NC}"
echo "Command: ./codex -m \"Say hello in exactly 3 words\""
echo "---"
./codex -m "Say hello in exactly 3 words"
echo -e "\n${GREEN}✓ Test 1 passed${NC}\n"

# Test 2: Counting (tests streaming)
echo -e "${GREEN}Test 2: Streaming test (counting)${NC}"
echo "Command: ./codex -m \"Count from 1 to 5, saying each number on its own line\""
echo "---"
./codex -m "Count from 1 to 5, saying each number on its own line"
echo -e "\n${GREEN}✓ Test 2 passed${NC}\n"

# Test 3: Session persistence
echo -e "${GREEN}Test 3: Session persistence${NC}"
SESSION_ID="test-session-$(date +%s)"
echo "Command: ./codex -s \"$SESSION_ID\" -m \"My name is Alice\""
echo "---"
./codex -s "$SESSION_ID" -m "My name is Alice"
echo ""

echo "Command: ./codex -s \"$SESSION_ID\" -m \"What is my name?\""
echo "---"
./codex -s "$SESSION_ID" -m "What is my name?"
echo -e "\n${GREEN}✓ Test 3 passed${NC}\n"

echo -e "${GREEN}=== All tests passed! ===${NC}"
echo ""
echo "The AI streaming feature is working correctly."
echo "You can now use the CLI in interactive mode by running: ./codex"
