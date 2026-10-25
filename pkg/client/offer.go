package client

import (
	"encoding/hex"
	"fmt"

	"github.com/arthneura/arthneura-market/internal/offersign"
)

type Offer struct {
	ID        int64  `json:"id"`
	ListingID int64  `json:"listing_id"`
	FromDid   string `json:"from_did"`
	ToDid     string `json:"to_did"`
	Price     int64  `json:"price"`
	Status    string `json:"status"`
	Schema    string `json:"schema,omitempty"`
}

type CreateOfferInput struct {
	ListingID int64
	FromDid   string
	ToDid     string
	Price     int64
	ExpiresAt int64
	Signature string
}

func OfferSignMessage(action string, listingOrOfferID int64, did string, price, expiresAt int64) []byte {
	return offersign.Message(action, listingOrOfferID, did, price, expiresAt)
}

func (c *Client) CreateOffer(in CreateOfferInput) (Offer, error) {
	if _, err := hex.DecodeString(in.Signature); err != nil || in.Signature == "" {
		return Offer{}, fmt.Errorf("signature must be hex")
	}
	var out Offer
	err := c.doJSON("POST", "/v1/offers", map[string]any{
		"listing_id": in.ListingID,
		"from_did":   in.FromDid,
		"to_did":     in.ToDid,
		"price":      in.Price,
		"expires_at": in.ExpiresAt,
		"signature":  in.Signature,
	}, &out)
	return out, err
}

type Stamp struct {
    OfferID      int64  `json:"offer_id"`
    ProviderDID  string `json:"provider_did"`
    ConsumerDID  string `json:"consumer_did"`
    Ready        bool   `json:"ready"`
    Reason       string `json:"reason"`
    Schema       string `json:"schema"`
    MerkleRoot   string `json:"merkle_root"`
    TotalChunks  int64  `json:"total_chunks"`
    CommitmentID string `json:"commitment_id"`
}

func (c *Client) Stamp(offerID int64) (Stamp, error) {
	var out Stamp
	err := c.doJSON("GET", fmt.Sprintf("/v1/offers/%d/stamp", offerID), nil, &out)
	return out, err
}
