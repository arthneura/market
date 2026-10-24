#!/bin/bash
set -euo pipefail

LOG_DIR="$HOME/.arthneura/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/decide-loop.log"

echo "[$(date)] starting continuous decide-loop" | tee -a "$LOG_FILE"

while true; do
  echo "[$(date)] running decide-loop" | tee -a "$LOG_FILE"
  ./scripts/decide-loop.sh >> "$LOG_FILE" 2>&1 || true
  sleep 45
done
