#!/bin/bash
set -euo pipefail

BIN="${BIN:-$HOME/.arthneura/bin/decide-me}"
LOCK_BIN="${LOCK_BIN:-$HOME/.arthneura/bin/lock-me}"
SPEC_BIN="${SPEC_BIN:-$HOME/.arthneura/bin/spec-me}"
COMMIT_BIN="${COMMIT_BIN:-$HOME/.arthneura/bin/commit-me}"
CHAIN_REGISTER_BIN="${CHAIN_REGISTER_BIN:-$HOME/.arthneura/bin/chain-register-me}"
POINTER="$HOME/.arthneura/owner.dir"
MARKET_URL="${MARKET_URL:-https://api.arthneura.com}"

[ -f "$POINTER" ] || exit 0
DIR="$(tr -d '[:space:]' < "$POINTER")"
[ -n "$DIR" ] || exit 0
[ -f "$DIR/owner.did" ] || exit 0
[ -f "$DIR/controller.seed" ] || exit 0

DID="$(tr -d '[:space:]' < "$DIR/owner.did")"
SEED="$(tr -d '[:space:]' < "$DIR/controller.seed")"

# 1. Accept only if rules.json exists (seller side)
if [ -f "$DIR/rules.json" ] && [ -x "$BIN" ]; then
  OWNER_DIR="$DIR" OWNER_DID="$DID" CONTROLLER_SEED="$SEED" \
    MARKET_URL="$MARKET_URL" "$BIN" || true
fi

# 2. Register / Spec / Commit / Lock for accepted offers
if command -v jq >/dev/null 2>&1; then
  OFFERS=$(curl -s "$MARKET_URL/v1/offers" || true)
  echo "$OFFERS" | jq -r '.offers[] | select(.status=="accepted") | .id' | while read -r OID; do
    [ -z "$OID" ] && continue
    STAMP=$(curl -s "$MARKET_URL/v1/offers/$OID/stamp" || true)
    CID=$(echo "$STAMP" | jq -r '.commitment_id // empty')
    READY=$(echo "$STAMP" | jq -r '.ready // false')
    MERKLE=$(echo "$STAMP" | jq -r '.merkle_root // empty')
    CHUNKS=$(echo "$STAMP" | jq -r '.total_chunks // empty')

    # Register if no commitment yet
    if [ -x "$CHAIN_REGISTER_BIN" ] && { [ -z "$CID" ] || [ "$CID" = "null" ]; }; then
      echo "AUTO-REGISTER offer=$OID"
      OWNER_DIR="$DIR" OFFER_ID="$OID" "$CHAIN_REGISTER_BIN" || true
    fi

    if [ -x "$SPEC_BIN" ] && [ -n "$MERKLE" ] && [ "$MERKLE" != "null" ] && [ -n "$CHUNKS" ] && [ "$CHUNKS" != "null" ]; then
      echo "AUTO-SPEC offer=$OID"
      OFFER_ID="$OID" MERKLE_ROOT="$MERKLE" TOTAL_CHUNKS="$CHUNKS" MARKET_URL="$MARKET_URL" "$SPEC_BIN" || true
    fi

    if [ -x "$COMMIT_BIN" ] && [ -n "$CID" ] && [ "$CID" != "null" ]; then
      echo "AUTO-COMMIT offer=$OID commitment=$CID"
      OFFER_ID="$OID" COMMITMENT_ID="$CID" MARKET_URL="$MARKET_URL" "$COMMIT_BIN" || true
    fi

    if [ -x "$LOCK_BIN" ] && [ -n "$CID" ] && [ "$CID" != "null" ] && [ "$READY" = "true" ]; then
      echo "AUTO-LOCK offer=$OID commitment=$CID"
      OWNER_DIR="$DIR" COMMITMENT_ID="$CID" "$LOCK_BIN" || true
    fi
  done
fi
