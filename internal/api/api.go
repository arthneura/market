package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/arthneura/arthneura-market/internal/announce"
    "github.com/arthneura/arthneura-market/internal/faucet"
	"github.com/arthneura/arthneura-market/internal/offersign"
	"github.com/arthneura/arthneura-market/internal/schema"
	"github.com/arthneura/arthneura-market/internal/store"
)

func NewMux(db *store.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /v1/agents", func(w http.ResponseWriter, r *http.Request) {
		agents, err := db.ListAgents(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if agents == nil {
			agents = []store.Agent{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
	})
	mux.HandleFunc("GET /v1/agents/{did}", func(w http.ResponseWriter, r *http.Request) {
		did := strings.TrimSpace(r.PathValue("did"))
		agent, err := db.GetAgent(r.Context(), did)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, agent)
	})
	mux.HandleFunc("GET /v1/commitments", func(w http.ResponseWriter, r *http.Request) {
		items, err := db.ListCommitments(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if items == nil {
			items = []store.Commitment{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"commitments": items})
	})
	mux.HandleFunc("GET /v1/commitments/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		item, err := db.GetCommitment(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "commitment not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("POST /v1/commitments/{id}/deliver", func(w http.ResponseWriter, r *http.Request) {
		idHex := strings.TrimSpace(r.PathValue("id"))
		id, err := hex.DecodeString(idHex)
		if err != nil || len(id) != 32 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad commitment id"})
			return
		}
		var body struct {
			URL       string `json:"url"`
			ExpiresAt int64  `json:"expires_at"`
			Signature string `json:"signature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
			return
		}
		item0, err := db.GetCommitment(r.Context(), idHex)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "commitment not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		agent, err := db.GetAgent(r.Context(), item0.Provider)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider agent not indexed"})
			return
		}
		ctrl, err := hex.DecodeString(agent.Controller)
		if err != nil || len(ctrl) != 32 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad controller"})
			return
		}
		sigb, err := hex.DecodeString(strings.TrimSpace(body.Signature))
		if err != nil || len(sigb) != 64 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "signature must be 64-byte hex"})
			return
		}
		var pub [32]byte
		var sig [64]byte
		copy(pub[:], ctrl)
		copy(sig[:], sigb)
		msg := announce.Message(idHex, strings.TrimSpace(body.URL), body.ExpiresAt)
		if err := announce.Verify(pub, sig, msg, body.ExpiresAt); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if err := db.SetDeliverURL(r.Context(), id, strings.TrimSpace(body.URL)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		item, err := db.GetCommitment(r.Context(), idHex)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("GET /v1/listings", func(w http.ResponseWriter, r *http.Request) {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		var cap int64
		if s := strings.TrimSpace(r.URL.Query().Get("cap")); s != "" {
			var err error
			cap, err = strconv.ParseInt(s, 10, 64)
			if err != nil || cap < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad cap"})
				return
			}
		}
		items, err := db.ListListings(r.Context(), status, cap)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if items == nil {
			items = []store.Listing{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"listings": items})
	})
	mux.HandleFunc("POST /v1/listings", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SellerDid string `json:"seller_did"`
			Title     string `json:"title"`
			Schema    string `json:"schema"`
			Price     int64  `json:"price"`
			ExpiresAt int64  `json:"expires_at"`
			Signature string `json:"signature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.Title = strings.TrimSpace(body.Title)
		if body.Title == "" || body.Price < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and non-negative price required"})
			return
		}
		seller, err := store.DecodeDid(body.SellerDid)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		agent, err := db.GetAgent(r.Context(), body.SellerDid)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "seller not registered"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if agent.Status != "" && agent.Status != "active" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "seller not active"})
			return
		}
		ctrl, err := hex.DecodeString(agent.Controller)
		if err != nil || len(ctrl) != 32 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad controller"})
			return
		}
		sigb, err := hex.DecodeString(strings.TrimSpace(body.Signature))
		if err != nil || len(sigb) != 64 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "signature must be 64-byte hex"})
			return
		}
		var pub [32]byte
		var sig [64]byte
		copy(pub[:], ctrl)
		copy(sig[:], sigb)
		if err := offersign.VerifyListing(pub, sig, body.SellerDid, body.Title, body.Price, body.ExpiresAt, body.Schema); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		schemaID, err := schema.Canonical(body.Schema)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		item, err := db.CreateListing(r.Context(), seller, body.Title, body.Price, schemaID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mountOffers(mux, db)
    mux.HandleFunc("POST /v1/faucet", faucet.Handler)
	mountNext(mux, db)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
