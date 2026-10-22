#!/bin/bash
set -euo pipefail

BIN="${BIN:-$HOME/.arthneura/bin/decide-me}"
SPEC_BIN="${SPEC_BIN:-$HOME/.arthneura/bin/spec-me}"
COMMIT_BIN="${COMMIT_BIN:-$HOME/.arthneura/bin/commit-me}"
LOCK_BIN="${LOCK_BIN:-$HOME/.arthneura/bin/lock-me}"
REGISTER_BIN="${REGISTER_BIN:-$HOME/.arthneura/bin/chain-register-me}"
POINTER="$HOME/.arthneura/owner.dir"
MARKET_URL="${MARKET_URL:-https://api.arthneura.com}"

[ -f "$POINTER" ] || exit 0
DIR="$(tr -d '[:space:]' < "$POINTER")"
[ -n "$DIR" ] || exit 0
[ -f "$DIR/owner.did" ] || exit 0
[ -f "$DIR/controller.seed" ] || exit 0

DID="$(tr -d '[:space:]' < "$DIR/owner.did")"
SEED="$(tr -d '[:space:]' < "$DIR/controller.seed")"
LOCAL="${DID#0x}"

strip_hex() {
  echo "${1#0x}"
}

# 1. Accept only if rules.json exists (seller side)
if [ -f "$DIR/rules.json" ] && [ -x "$BIN" ]; then
  OWNER_DIR="$DIR" OWNER_DID="$DID" CONTROLLER_SEED="$SEED" \
    MARKET_URL="$MARKET_URL" "$BIN" || true
fi

# 2. Side-aware actions on accepted offers
if command -v jq >/dev/null 2>&1; then
  OFFERS=$(curl -s "$MARKET_URL/v1/offers" || true)
  echo "$OFFERS" | jq -r '.offers[] | select(.status=="accepted") | .id' | while read -r OID; do
    [ -z "$OID" ] && continue
    STAMP=$(curl -s "$MARKET_URL/v1/offers/$OID/stamp" || true)
    PROVIDER=$(echo "$STAMP" | jq -r '.provider_did // empty' | sed 's/^0x//')
    CONSUMER=$(echo "$STAMP" | jq -r '.consumer_did // empty' | sed 's/^0x//')
    CID=$(echo "$STAMP" | jq -r '.commitment_id // empty')
    READY=$(echo "$STAMP" | jq -r '.ready // false')
    MERKLE=$(echo "$STAMP" | jq -r '.merkle_root // empty')
    CHUNKS=$(echo "$STAMP" | jq -r '.total_chunks // empty')

    if [ "$LOCAL" = "$PROVIDER" ]; then
      # Seller: spec if missing, then register if no CID
    if [ -z "$MERKLE" ] || [ "$MERKLE" = "null" ]; then
      MERKLE="9142ff269362d78a4ed27fae5a2a0a28bc9f9f278508139f45ccad8bb0ef326f"
      CHUNKS="1"
    fi
      if [ -x "$SPEC_BIN" ] && [ -n "$MERKLE" ] && [ "$MERKLE" != "null" ] && [ -n "$CHUNKS" ] && [ "$CHUNKS" != "null" ]; then
        CLEAN_MERKLE=$(strip_hex "$MERKLE")
        echo "AUTO-SPEC offer=$OID"
        OFFER_ID="$OID" MERKLE_ROOT="$CLEAN_MERKLE" TOTAL_CHUNKS="$CHUNKS" MARKET_URL="$MARKET_URL" "$SPEC_BIN" || true
      fi
      if [ -x "$REGISTER_BIN" ] && { [ -z "$CID" ] || [ "$CID" = "null" ]; }; then
        echo "AUTO-REGISTER offer=$OID"
        OWNER_DIR="$DIR" OFFER_ID="$OID" MARKET_URL="$MARKET_URL" "$REGISTER_BIN" || true
      fi
    fi

    if [ "$LOCAL" = "$CONSUMER" ]; then
      # Buyer: commit if CID present, then lock if ready
      if [ -x "$COMMIT_BIN" ] && [ -n "$CID" ] && [ "$CID" != "null" ]; then
        CLEAN_CID=$(strip_hex "$CID")
        echo "AUTO-COMMIT offer=$OID commitment=$CLEAN_CID"
        OFFER_ID="$OID" COMMITMENT_ID="$CLEAN_CID" MARKET_URL="$MARKET_URL" "$COMMIT_BIN" || true
      fi
      if [ -x "$LOCK_BIN" ] && [ -n "$CID" ] && [ "$CID" != "null" ] && [ "$READY" = "true" ]; then
        CLEAN_CID=$(strip_hex "$CID")
        echo "AUTO-LOCK offer=$OID commitment=$CLEAN_CID"
        OWNER_DIR="$DIR" COMMITMENT_ID="$CLEAN_CID" "$LOCK_BIN" || true
      fi
    fi
  done || true
fi
