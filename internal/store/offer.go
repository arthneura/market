package store

import (
    "context"
    "encoding/hex"
    "fmt"
    "time"
)

type Offer struct {
    ID              int64  `json:"id"`
    ListingID       int64  `json:"listing_id"`
    FromDid         string `json:"from_did"`
    ToDid           string `json:"to_did"`
    Price           int64  `json:"price"`
    ExpiresAt       string `json:"expires_at"`
    Status          string `json:"status"`
    MerkleRoot      string `json:"merkle_root,omitempty"`
    TotalChunks     int64  `json:"total_chunks,omitempty"`
    ExpiresInBlocks int64  `json:"expires_in_blocks,omitempty"`
	CommitmentID    string `json:"commitment_id,omitempty"`
    Schema          string `json:"schema,omitempty"`
}

func (s *Store) CreateOffer(ctx context.Context, listingID int64, from, to []byte, price int64, exp time.Time) (Offer, error) {
    listing, err := s.GetListing(ctx, listingID)
    if err != nil {
        return Offer{}, err
    }
    var id int64
    err = s.pool.QueryRow(ctx, `
        INSERT INTO offers (listing_id, from_did, to_did, price, expires_at, status, schema)
        VALUES ($1,$2,$3,$4,$5,'open',$6)
        RETURNING id
    `, listingID, from, to, price, exp, listing.Schema).Scan(&id)
    if err != nil {
        return Offer{}, err
    }
    return Offer{
        ID: id, ListingID: listingID,
        FromDid: hex.EncodeToString(from), ToDid: hex.EncodeToString(to),
        Price: price, ExpiresAt: exp.UTC().Format(time.RFC3339), Status: "open",
        Schema: listing.Schema,
    }, nil
}

func (s *Store) CancelOffer(ctx context.Context, id int64) error {
    _, err := s.pool.Exec(ctx, `
        UPDATE offers SET status = 'cancelled'
        WHERE id = $1 AND status = 'open'
    `, id)
    return err
}

func scanOffer(id, listingID, price, chunks, blocks int64, from, to, root, cid []byte, exp time.Time, status, schema string) Offer {
    rootHex := ""
    if len(root) > 0 {
        rootHex = hex.EncodeToString(root)
    }
    cidHex := ""
    if len(cid) > 0 {
        cidHex = hex.EncodeToString(cid)
    }
    return Offer{
        ID: id, ListingID: listingID,
        FromDid: hex.EncodeToString(from), ToDid: hex.EncodeToString(to),
        Price: price, ExpiresAt: exp.UTC().Format(time.RFC3339), Status: status,
        MerkleRoot: rootHex, TotalChunks: chunks, ExpiresInBlocks: blocks, CommitmentID: cidHex,
        Schema: schema,
    }
}

func (s *Store) ListOffers(ctx context.Context) ([]Offer, error) {
    _, _ = s.pool.Exec(ctx, `
        UPDATE offers SET status = 'expired'
        WHERE status = 'open' AND expires_at < now()
    `)
    rows, err := s.pool.Query(ctx, `
        SELECT id, listing_id, from_did, to_did, price, expires_at, status,
               merkle_root, COALESCE(total_chunks,0), COALESCE(expires_in_blocks,0),
               commitment_id, COALESCE(schema,'')
        FROM offers
        ORDER BY id DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []Offer
    for rows.Next() {
        var id, listingID, price, chunks, blocks int64
        var from, to, root, cid []byte
        var exp time.Time
        var status string
        var schema string
        if err := rows.Scan(&id, &listingID, &from, &to, &price, &exp, &status, &root, &chunks, &blocks, &cid, &schema); err != nil {
            return nil, err
        }
        out = append(out, scanOffer(id, listingID, price, chunks, blocks, from, to, root, cid, exp, status, schema))
    }
    return out, rows.Err()
}

func (s *Store) GetOffer(ctx context.Context, id int64) (Offer, error) {
    var listingID, price, chunks, blocks int64
    var from, to, root, cid []byte
    var exp time.Time
    var status string
    var schema string
    err := s.pool.QueryRow(ctx, `
        SELECT id, listing_id, from_did, to_did, price, expires_at, status,
               merkle_root, COALESCE(total_chunks,0), COALESCE(expires_in_blocks,0),
               commitment_id, COALESCE(schema,'')
        FROM offers WHERE id = $1
    `, id).Scan(&id, &listingID, &from, &to, &price, &exp, &status, &root, &chunks, &blocks, &cid, &schema)
    if err != nil {
        return Offer{}, err
    }
    return scanOffer(id, listingID, price, chunks, blocks, from, to, root, cid, exp, status, schema), nil
}

func (s *Store) CounterOffer(ctx context.Context, oldID int64, price int64, exp time.Time) (Offer, error) {
    old, err := s.GetOffer(ctx, oldID)
    if err != nil {
        return Offer{}, err
    }
    if old.Status != "open" {
        return Offer{}, fmt.Errorf("offer not open")
    }
    _, err = s.pool.Exec(ctx, `UPDATE offers SET status = 'countered' WHERE id = $1 AND status = 'open'`, oldID)
    if err != nil {
        return Offer{}, err
    }
    from, err := DecodeDid(old.ToDid)
    if err != nil {
        return Offer{}, err
    }
    to, err := DecodeDid(old.FromDid)
    if err != nil {
        return Offer{}, err
    }
    return s.CreateOffer(ctx, old.ListingID, from, to, price, exp)
}

func (s *Store) AcceptOffer(ctx context.Context, id int64, who []byte) (Offer, error) {
    var from, to []byte
    var accFrom, accTo bool
    var status string
    err := s.pool.QueryRow(ctx, `
        SELECT from_did, to_did, accepted_from, accepted_to, status
        FROM offers WHERE id = $1
    `, id).Scan(&from, &to, &accFrom, &accTo, &status)
    if err != nil {
        return Offer{}, err
    }
    if status != "open" {
        return Offer{}, fmt.Errorf("offer not open")
    }
    same := func(a, b []byte) bool {
        if len(a) != len(b) {
            return false
        }
        for i := range a {
            if a[i] != b[i] {
                return false
            }
        }
        return true
    }
    if same(who, from) {
        accFrom = true
    } else if same(who, to) {
        accTo = true
    } else {
        return Offer{}, fmt.Errorf("not a party")
    }
    st := "open"
    // Seller accept is enough
    if accTo {
        st = "accepted"
    } else if accFrom && accTo {
        st = "accepted"
    }
    _, err = s.pool.Exec(ctx, `
        UPDATE offers SET accepted_from = $2, accepted_to = $3, status = $4
        WHERE id = $1
    `, id, accFrom, accTo, st)
    if err != nil {
        return Offer{}, err
    }
    return s.GetOffer(ctx, id)
}

func (s *Store) SetOfferSpec(ctx context.Context, id int64, root []byte, chunks, blocks int64) error {
    _, err := s.pool.Exec(ctx, `
        UPDATE offers
        SET merkle_root = $2, total_chunks = $3, expires_in_blocks = $4
        WHERE id = $1 AND status IN ('open','accepted')
    `, id, root, chunks, blocks)
    return err
}


func (s *Store) SetOfferCommitment(ctx context.Context, offerID int64, commitmentID []byte) error {
    if len(commitmentID) != 32 {
        return fmt.Errorf("commitment id must be 32 bytes")
    }
    tag, err := s.pool.Exec(ctx, `
        UPDATE offers SET commitment_id = $2
        WHERE id = $1 AND status = 'accepted'
    `, offerID, commitmentID)
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return fmt.Errorf("offer not accepted or missing")
    }
    return nil
}


func (s *Store) LinkOfferByParties(ctx context.Context, provider, consumer, commitmentID, root []byte, chunks int64) error {
    if len(commitmentID) != 32 {
        return fmt.Errorf("commitment id must be 32 bytes")
    }
    tag, err := s.pool.Exec(ctx, `
        UPDATE offers SET commitment_id = $3, merkle_root = $4, total_chunks = $5
        WHERE id = (
            SELECT id FROM offers
            WHERE status = 'accepted'
              AND commitment_id IS NULL
              AND (
                    (from_did = $1 AND to_did = $2)
                 OR (from_did = $2 AND to_did = $1)
              )
            ORDER BY id DESC
            LIMIT 1
        )
    `, provider, consumer, commitmentID, root, chunks)
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return fmt.Errorf("no accepted offer for these parties")
    }
    return nil
}
