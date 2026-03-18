#!/bin/bash
set -e

URL=""
KEY=""
PLATFORM="claude"

while [[ $# -gt 0 ]]; do
  case $1 in
    --url) URL="$2"; shift 2;;
    --key) KEY="$2"; shift 2;;
    --platform) PLATFORM="$2"; shift 2;;
    *) echo "Unknown option: $1"; exit 1;;
  esac
done

if [ -z "$URL" ] || [ -z "$KEY" ]; then
  echo "Usage: curl -s <server>/setup.sh | bash -s -- --url <url> --key <key> [--platform claude|codex]"
  exit 1
fi

if [ "$PLATFORM" = "claude" ]; then
  # Detect shell config file
  SHELL_RC="$HOME/.zshrc"
  if [ ! -f "$SHELL_RC" ]; then
    SHELL_RC="$HOME/.bashrc"
  fi
  touch "$SHELL_RC"

  # Remove existing entries
  TMP_FILE=$(mktemp)
  grep -v '^export ANTHROPIC_AUTH_TOKEN=' "$SHELL_RC" 2>/dev/null | grep -v '^export ANTHROPIC_BASE_URL=' > "$TMP_FILE" || true
  mv "$TMP_FILE" "$SHELL_RC"

  # Append new values
  echo "export ANTHROPIC_AUTH_TOKEN=\"$KEY\"" >> "$SHELL_RC"
  echo "export ANTHROPIC_BASE_URL=\"$URL\"" >> "$SHELL_RC"

  # Set for current session
  export ANTHROPIC_AUTH_TOKEN="$KEY"
  export ANTHROPIC_BASE_URL="$URL"

  echo "✅ Claude Code configured successfully!"
  echo "   ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL written to $SHELL_RC"
  echo "   Run 'source $SHELL_RC' or restart your terminal to apply."

elif [ "$PLATFORM" = "codex" ]; then
  CODEX_DIR="$HOME/.codex"
  mkdir -p "$CODEX_DIR"

  cat > "$CODEX_DIR/config.toml" << TOMLEOF
model_provider = "MikuCode"
model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
preferred_auth_method = "apikey"
[model_providers.MikuCode]
name = "MikuCode"
base_url = "${URL}/v1"
wire_api = "responses"
requires_openai_auth = true
TOMLEOF

  cat > "$CODEX_DIR/auth.json" << JSONEOF
{
  "OPENAI_API_KEY": "$KEY"
}
JSONEOF

  echo "✅ Codex configured successfully!"
  echo "   Config written to $CODEX_DIR/config.toml"
  echo "   Auth written to $CODEX_DIR/auth.json"

else
  echo "Unknown platform: $PLATFORM (use 'claude' or 'codex')"
  exit 1
fi
