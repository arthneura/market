#!/bin/bash
set -euo pipefail

BIN="${BIN:-$HOME/.arthneura/bin/decide-me}"
LOCK_BIN="${LOCK_BIN:-$HOME/.arthneura/bin/lock-me}"
POINTER="$HOME/.arthneura/owner.dir"
MARKET_URL="${MARKET_URL:-https://api.arthneura.com}"

[ -x "$BIN" ] || exit 0
[ -f "$POINTER" ] || exit 0
DIR="$(tr -d '[:space:]' < "$POINTER")"
[ -n "$DIR" ] || exit 0
[ -f "$DIR/rules.json" ] || exit 0
[ -f "$DIR/owner.did" ] || exit 0
[ -f "$DIR/controller.seed" ] || exit 0

DID="$(tr -d '[:space:]' < "$DIR/owner.did")"
SEED="$(tr -d '[:space:]' < "$DIR/controller.seed")"

# 1. Accept best offers above floor
OWNER_DIR="$DIR" OWNER_DID="$DID" CONTROLLER_SEED="$SEED" \
  MARKET_URL="$MARKET_URL" "$BIN"

# 2. Auto-lock if commitment already exists for accepted offers
if [ -x "$LOCK_BIN" ]; then
  # Get accepted offers (simple curl + jq if available)
  OFFERS=$(curl -s "$MARKET_URL/v1/offers" || true)
  if command -v jq >/dev/null 2>&1 && [ -n "$OFFERS" ]; then
    echo "$OFFERS" | jq -r '.offers[] | select(.status=="accepted") | .id' | while read -r OID; do
      [ -z "$OID" ] && continue
      STAMP=$(curl -s "$MARKET_URL/v1/offers/$OID/stamp" || true)
      CID=$(echo "$STAMP" | jq -r '.commitment_id // empty')
      READY=$(echo "$STAMP" | jq -r '.ready // false')
      if [ -n "$CID" ] && [ "$CID" != "null" ] && [ "$READY" = "true" ]; then
        echo "AUTO-LOCK offer=$OID commitment=$CID"
        OWNER_DIR="$DIR" COMMITMENT_ID="$CID" "$LOCK_BIN" || true
      fi
    done
  fi
fi
