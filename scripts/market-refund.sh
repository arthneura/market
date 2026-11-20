#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CORE="${ARTHNEURA_CORE:-$HOME/arthneura-core}"
cd "$ROOT"
curl -sf http://127.0.0.1:8080/health >/dev/null || { echo "ERROR api down"; exit 1; }
docker ps --format "{{.Names}}" | grep -q "^arthneura-dev-node$" || { echo "ERROR node down"; exit 1; }
cd "$CORE"
P_LINE="$(SIGNER=alice LABEL=provider cargo run -q -p offchain-agent-registry)"
C_LINE="$(SIGNER=bob LABEL=consumer cargo run -q -p offchain-agent-registry)"
PROVIDER_DID="$(echo "$P_LINE" | sed -n "s/^DID=0x//p" | tail -n 1)"
CONSUMER_DID="$(echo "$C_LINE" | sed -n "s/^DID=0x//p" | tail -n 1)"
sleep 6
cd "$ROOT"
EXP=$(( $(date +%s) + 3600 ))
LIST_JSON="$(SIGNER=alice go run ./cmd/offer-sign -action listing -did "$PROVIDER_DID" -title "csv leads bad" -price 1000 -exp "$EXP")"
SIG="$(echo "$LIST_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"signature\"])")"
L="$(curl -sf -X POST http://127.0.0.1:8080/v1/listings -H "Content-Type: application/json" -d "{\"seller_did\":\"$PROVIDER_DID\",\"title\":\"csv leads bad\",\"price\":1000,\"expires_at\":$EXP,\"signature\":\"$SIG\"}")"
LID="$(echo "$L" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"id\"])")"
OFF_JSON="$(SIGNER=bob go run ./cmd/offer-sign -action create -id "$LID" -did "$CONSUMER_DID" -price 1000 -exp "$EXP")"
SIG="$(echo "$OFF_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"signature\"])")"
O="$(curl -sf -X POST http://127.0.0.1:8080/v1/offers -H "Content-Type: application/json" -d "{\"listing_id\":$LID,\"from_did\":\"$CONSUMER_DID\",\"to_did\":\"$PROVIDER_DID\",\"price\":1000,\"expires_at\":$EXP,\"signature\":\"$SIG\"}")"
OID="$(echo "$O" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"id\"])")"
for PAIR in "alice:$PROVIDER_DID" "bob:$CONSUMER_DID"; do
  WHO="${PAIR%%:*}"; DID="${PAIR##*:}"
  AJ="$(SIGNER=$WHO go run ./cmd/offer-sign -action accept -id "$OID" -did "$DID" -price 1000 -exp "$EXP")"
  SIG="$(echo "$AJ" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"signature\"])")"
  curl -sf -X POST "http://127.0.0.1:8080/v1/offers/$OID/accept" -H "Content-Type: application/json" -d "{\"did\":\"$DID\",\"price\":1000,\"expires_at\":$EXP,\"signature\":\"$SIG\"}" >/dev/null
done
ROOTHEX="be373c8f3e5339301844a332bbe22f095d937fcd3753033ddbdce75f830b371e"
curl -sf -X POST "http://127.0.0.1:8080/v1/offers/$OID/spec" -H "Content-Type: application/json" -d "{\"merkle_root\":\"$ROOTHEX\",\"total_chunks\":1,\"expires_in_blocks\":1000}" >/dev/null
cd "$CORE"
REG="$(ACTION=register SIGNER=alice PROVIDER_DID="0x$PROVIDER_DID" CONSUMER_DID="0x$CONSUMER_DID" PRICE=1000 PAYLOAD="hello arthneura" cargo run -q -p offchain-vector-db)"
CID="$(echo "$REG" | sed -n "s/^COMMITMENT_ID=0x//p" | tail -n 1)"
cd "$ROOT"
curl -sf -X POST "http://127.0.0.1:8080/v1/offers/$OID/commitment" -H "Content-Type: application/json" -d "{\"commitment_id\":\"$CID\"}" >/dev/null
cd "$CORE"
ACTION=acknowledge SIGNER=bob COMMITMENT_ID="0x$CID" CONSUMER_DID="0x$CONSUMER_DID" cargo run -q -p offchain-vector-db
ACTION=raise SIGNER=bob COMMITMENT_ID="0x$CID" CONSUMER_DID="0x$CONSUMER_DID" CHUNK_INDEX=0 TOTAL_CHUNKS=1 RECEIVED_CHUNK_HASH=0000000000000000000000000000000000000000000000000000000000000001 cargo run -q -p offchain-vector-db
echo "=== wait window ==="
sleep 90
ACTION=finalize SIGNER=bob COMMITMENT_ID="0x$CID" cargo run -q -p offchain-vector-db
sleep 6
cd "$ROOT"
curl -sf "http://127.0.0.1:8080/v1/commitments/$CID"
echo
echo "OFFER_ID=$OID"
echo "COMMITMENT_ID=$CID"
echo "RESULT=MARKET_REFUNDED"
