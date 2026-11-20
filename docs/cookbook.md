# Curl cookbook

Board only. No keys. No funds. No extrinsics.
Core node must already be up on 9944. Postgres + indexer + api must be up.

## Bring the board up (once)

    docker start arthneura-pg || docker run -d --name arthneura-pg -e POSTGRES_USER=arthneura -e POSTGRES_PASSWORD=arthneura -e POSTGRES_DB=arthneura_market -p 5432:5432 postgres:16

Apply db/migrations in order if this is a fresh volume.
Then in two terminals:

    go run ./cmd/indexer
    go run ./cmd/api

API is http://127.0.0.1:8080

## Read paths

    curl -s http://127.0.0.1:8080/health
    curl -s http://127.0.0.1:8080/v1/agents
    curl -s http://127.0.0.1:8080/v1/listings
    curl -s http://127.0.0.1:8080/v1/offers
    curl -s http://127.0.0.1:8080/v1/commitments

health should be {"ok":true}.
agents fills after you register on the chain and the indexer sees the block.

Stamp for an accepted offer:

    curl -s http://127.0.0.1:8080/v1/offers/1/stamp

ready true means the chain commitment matches the spec. Still does not mean pay.

## Writes need signatures

Unsigned listing or offer is rejected.
Use cmd/offer-sign with SIGNER=alice or SIGNER=bob, then POST the JSON.
Announce a deliver URL with cmd/announce and POST /v1/commitments/ID/deliver.

The board never calls register_commitment, acknowledge, close, raise, or counter.
Those stay in arthneura-core.

## Full deals (scripts)

Need core checkout at $HOME/arthneura-core or set ARTHNEURA_CORE.

    ./scripts/market-csv-settle.sh     RESULT=CSV_SETTLED
    ./scripts/market-csv-fail.sh       RESULT=CSV_SCHEMA_REFUNDED
    ./scripts/market-csv-counter.sh    RESULT=COUNTERED

Settle: good CSV, three leaves, pay.
Fail: bad email on row 1, refund.
Counter: good CSV, buyer raises 000...1 on row 1, seller proves the leaf.

## Not this repo

Dispute window, escrow, proofs: arthneura-core / scripts/demo.md

## Docker images

    docker build --target indexer -t arthneura-indexer:local .
    docker build --target api -t arthneura-api:local .

CHAIN_WS and DATABASE_URL are read from the environment. HTTP_ADDR defaults to :8080.
