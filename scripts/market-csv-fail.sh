#!/usr/bin/env bash
set -euo pipefail
DELIVER_URL="${DELIVER_URL:-http://127.0.0.1:8090}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CORE="${ARTHNEURA_CORE:-$HOME/arthneura-core}"
CSV="$ROOT/testdata/csv/bad-email.csv"
cd "$ROOT"
curl -sf http://127.0.0.1:8080/health >/dev/null || { echo "ERROR api down"; exit 1; }
set +e
CHK="$(go run ./cmd/csvcheck -schema csv.v1 "$CSV" 2>&1)"
CHK_OK=$?
set -e
echo "$CHK"
[ "$CHK_OK" -ne 0 ] || { echo "ERROR csv should fail"; exit 1; }
cd "$CORE"
ALICE_DIR="${ALICE_DIR:-$HOME/agents/alice}"
BOB_DIR="${BOB_DIR:-$HOME/agents/bob}"
mkdir -p "$ALICE_DIR" "$BOB_DIR"
P_LINE="$(KEYSTORE_DIR="$ALICE_DIR" KEY_LABEL=alice SIGNER=alice LABEL=provider cargo run -q -p offchain-agent-registry)"
C_LINE="$(KEYSTORE_DIR="$BOB_DIR" KEY_LABEL=bob SIGNER=bob LABEL=consumer cargo run -q -p offchain-agent-registry)"
PROVIDER_DID="$(echo "$P_LINE" | sed -n "s/^DID=0x//p" | tail -n 1)"
CONSUMER_DID="$(echo "$C_LINE" | sed -n "s/^DID=0x//p" | tail -n 1)"
sleep 6
cd "$ROOT"
EXP=$(( $(date +%s) + 3600 ))
LIST_JSON="$(SIGNER=alice go run ./cmd/offer-sign -action listing -did "$PROVIDER_DID" -title "csv schema v1" -schema csv.v1 -price 1000 -exp "$EXP")"
SIG="$(echo "$LIST_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"signature\"])")"
L="$(curl -sf -X POST http://127.0.0.1:8080/v1/listings -H "Content-Type: application/json" -d "{\"seller_did\":\"$PROVIDER_DID\",\"title\":\"csv schema v1\",\"schema\":\"csv.v1\",\"price\":1000,\"expires_at\":$EXP,\"signature\":\"$SIG\"}")"
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
PAYLOAD="$(cat "$CSV")"
cd "$CORE"
REG="$(ACTION=register SIGNER=alice PROVIDER_DID="0x$PROVIDER_DID" CONSUMER_DID="0x$CONSUMER_DID" PRICE=1000 CHUNK_MODE=rows PAYLOAD="$PAYLOAD" cargo run -q -p offchain-vector-db)"
CID="$(echo "$REG" | sed -n "s/^COMMITMENT_ID=0x//p" | tail -n 1)"
ROOTHEX="$(echo "$REG" | sed -n "s/^MERKLE_ROOT=0x//p" | tail -n 1)"
CHUNKS="$(echo "$REG" | sed -n "s/^TOTAL_CHUNKS=//p" | tail -n 1)"
cd "$ROOT"
curl -sf -X POST "http://127.0.0.1:8080/v1/offers/$OID/spec" -H "Content-Type: application/json" -d "{\"merkle_root\":\"$ROOTHEX\",\"total_chunks\":$CHUNKS,\"expires_in_blocks\":1000}" >/dev/null
curl -sf -X POST "http://127.0.0.1:8080/v1/offers/$OID/commitment" -H "Content-Type: application/json" -d "{\"commitment_id\":\"$CID\"}" >/dev/null
cd "$CORE"
ACTION=acknowledge SIGNER=bob COMMITMENT_ID="0x$CID" CONSUMER_DID="0x$CONSUMER_DID" cargo run -q -p offchain-vector-db
cd "$ROOT"
CHUNK_MODE=rows PAYLOAD="$PAYLOAD" go run ./cmd/provider -offer "$OID" &
PROV_PID=$!
sleep 2
AN="$(SIGNER=alice go run ./cmd/announce -id "$CID" -url "$DELIVER_URL" -exp "$EXP")"
SIG="$(echo "$AN" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"signature\"])")"
curl -sf -X POST "http://127.0.0.1:8080/v1/commitments/$CID/deliver" -H "Content-Type: application/json" -d "{\"url\":\"$DELIVER_URL\",\"expires_at\":$EXP,\"signature\":\"$SIG\"}" >/dev/null
PULL_OUT="$(go run ./cmd/pull -offer "$OID" 2>&1)"
echo "$PULL_OUT"
RECV="$(echo "$PULL_OUT" | sed -n "s/^RECEIVED_FILE=//p" | tail -n 1)"
HASH0="$(echo "$PULL_OUT" | sed -n "s/^CHUNK_HASH_0=0x//p" | tail -n 1)"
[ -n "$RECV" ] && [ -n "$HASH0" ] || { echo "ERROR evidence missing"; echo "$PULL_OUT"; exit 1; }
set +e
CHK="$(go run ./cmd/csvcheck -schema csv.v1 "$RECV" 2>&1)"
CSV_OK=$?
set -e
echo "$CHK"
kill $PROV_PID >/dev/null 2>&1 || true
[ "$CSV_OK" -ne 0 ] || { echo "ERROR schema should fail"; exit 1; }
ROW="$(echo "$CHK" | sed -n "s/^ROW=//p" | tail -n 1 | awk "{print \$1}")"
[ -n "$ROW" ] || ROW=1
HASH="$(echo "$PULL_OUT" | sed -n "s/^CHUNK_HASH_${ROW}=0x//p" | tail -n 1)"
[ -n "$HASH" ] || HASH="$HASH0"
echo "RAISE_INDEX=$ROW"
echo "RAISE_HASH=0x$HASH"
cd "$CORE"
ACTION=raise SIGNER=bob COMMITMENT_ID="0x$CID" CONSUMER_DID="0x$CONSUMER_DID" CHUNK_INDEX="$ROW" TOTAL_CHUNKS="$CHUNKS" RECEIVED_CHUNK_HASH="$HASH" cargo run -q -p offchain-vector-db
echo "=== wait window ==="
sleep 90
ACTION=finalize SIGNER=bob COMMITMENT_ID="0x$CID" cargo run -q -p offchain-vector-db
sleep 6
cd "$ROOT"
curl -sf "http://127.0.0.1:8080/v1/commitments/$CID"
echo
echo "RESULT=CSV_SCHEMA_REFUNDED"
