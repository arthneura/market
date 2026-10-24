#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CORE="${ARTHNEURA_CORE:-$HOME/arthneura-core}"
BIN_DIR="$HOME/.arthneura/bin"
mkdir -p "$BIN_DIR"
if [ ! -x "$CORE/target/debug/register-me" ]; then
  echo "building register-me"
  (cd "$CORE" && cargo build -p offchain-agent-registry --bin register-me)
fi
cp "$CORE/target/debug/register-me" "$BIN_DIR/register-me"
(cd "$ROOT" && go build -o "$BIN_DIR/arthneura-mcp" ./cmd/arthneura-mcp)
chmod 755 "$BIN_DIR/register-me" "$BIN_DIR/arthneura-mcp"
case "$(uname -s)" in
  Darwin) CFG="$HOME/Library/Application Support/Claude/claude_desktop_config.json" ;;
  Linux)  CFG="$HOME/.config/Claude/claude_desktop_config.json" ;;
  *) echo "Windows: run install-desktop.ps1"; exit 1 ;;
esac
mkdir -p "$(dirname "$CFG")"
python3 - "$CFG" "$BIN_DIR" << 'PY'
import json, sys
from pathlib import Path
cfg, bindir = Path(sys.argv[1]), Path(sys.argv[2])
data = {}
if cfg.exists() and cfg.read_text().strip():
    data = json.loads(cfg.read_text())
data.setdefault("mcpServers", {})
data["mcpServers"]["arthneura"] = {
    "command": str(bindir / "arthneura-mcp"),
    "args": [],
    "env": {
        "MARKET_URL": "https://api.arthneura.com",
        "DOOR": "https://id.arthneura.com",
        "REGISTER_BIN": str(bindir / "register-me"),
        "KEYSTORE_PASS": "dev-passphrase",
    },
}
cfg.write_text(json.dumps(data, indent=2) + "\n")
print("WROTE", cfg)
print("MCP", bindir / "arthneura-mcp")
print("REGISTER", bindir / "register-me")
PY

# --- Start continuous decide-loop (macOS) ---
if [ "$(uname -s)" = "Darwin" ]; then
  PLIST_SRC="$ROOT/scripts/launchd/com.arthneura.decide-loop.plist"
  PLIST_DST="$HOME/Library/LaunchAgents/com.arthneura.decide-loop.plist"
  mkdir -p "$HOME/Library/LaunchAgents" "$HOME/.arthneura/logs"
  
  # Replace placeholder with actual home
  sed "s|/Users/REPLACE_ME|$HOME|g" "$PLIST_SRC" > "$PLIST_DST"
  
  # Load the service
  launchctl unload "$PLIST_DST" 2>/dev/null || true
  launchctl load "$PLIST_DST"
  echo "Loaded continuous decide-loop service"
fi
