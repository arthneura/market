package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arthneura/arthneura-market/pkg/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func marketURL() string {
	u := os.Getenv("MARKET_URL")
	if u == "" {
		u = "http://127.0.0.1:8080"
	}
	return u
}

func market() *client.Client { return client.New(marketURL()) }

type emptyIn struct{}

type healthOut struct {
	Ok     bool   `json:"ok"`
	Market string `json:"market"`
	Error  string `json:"error,omitempty"`
}

func health(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, healthOut, error) {
	u := marketURL()
	if err := market().Health(); err != nil {
		return nil, healthOut{Ok: false, Market: u, Error: err.Error()}, nil
	}
	return nil, healthOut{Ok: true, Market: u}, nil
}

type stampIn struct {
	OfferID int64 `json:"offer_id" jsonschema:"accepted offer id"`
}

type stampOut struct {
	OfferID      int64  `json:"offer_id"`
	Ready        bool   `json:"ready"`
	Reason       string `json:"reason"`
	Schema       string `json:"schema"`
	CommitmentID string `json:"commitment_id"`
	Error        string `json:"error,omitempty"`
}

func stamp(_ context.Context, _ *mcp.CallToolRequest, in stampIn) (*mcp.CallToolResult, stampOut, error) {
	if in.OfferID <= 0 {
		return nil, stampOut{Error: "offer_id must be > 0"}, nil
	}
	st, err := market().Stamp(in.OfferID)
	if err != nil {
		return nil, stampOut{OfferID: in.OfferID, Error: err.Error()}, nil
	}
	return nil, stampOut{
		OfferID: st.OfferID, Ready: st.Ready, Reason: st.Reason,
		Schema: st.Schema, CommitmentID: st.CommitmentID,
	}, nil
}

type listingsOut struct {
	Error string           `json:"error,omitempty"`
	N     int              `json:"n"`
	Items []client.Listing `json:"items"`
}

type offersOut struct {
	Error string         `json:"error,omitempty"`
	N     int            `json:"n"`
	Items []client.Offer `json:"items"`
}

func listListings(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, listingsOut, error) {
	items, err := market().ListListings()
	if err != nil {
		return nil, listingsOut{Error: err.Error()}, nil
	}
	return nil, listingsOut{N: len(items), Items: items}, nil
}

func listOffers(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, offersOut, error) {
	items, err := market().ListOffers()
	if err != nil {
		return nil, offersOut{Error: err.Error()}, nil
	}
	return nil, offersOut{N: len(items), Items: items}, nil
}

type commitIn struct {
	ID string `json:"id" jsonschema:"commitment id hex without 0x"`
}

func getCommitment(_ context.Context, _ *mcp.CallToolRequest, in commitIn) (*mcp.CallToolResult, client.Commitment, error) {
	item, err := market().GetCommitment(in.ID)
	if err != nil {
		return nil, client.Commitment{}, err
	}
	return nil, item, nil
}

type agentIn struct {
	DID string `json:"did" jsonschema:"agent DID hex without 0x"`
}

func getAgent(_ context.Context, _ *mcp.CallToolRequest, in agentIn) (*mcp.CallToolResult, client.Agent, error) {
	item, err := market().GetAgent(in.DID)
	if err != nil {
		return nil, client.Agent{}, err
	}
	return nil, item, nil
}

type registerIn struct {
	Label string `json:"label" jsonschema:"display label, ASCII letters numbers dash underscore"`
}

type registerOut struct {
	DID   string `json:"did,omitempty"`
	Key   string `json:"key,omitempty"`
	Error string `json:"error,omitempty"`
}

type listIn struct {
	Title  string `json:"title" jsonschema:"what to sell"`
	Schema string `json:"schema" jsonschema:"csv.v1 bytes.v1 api.v1 job.v1 or meter.v1"`
	Price  int64  `json:"price" jsonschema:"price, must be > 0"`
}

type listOut struct {
	Error     string `json:"error,omitempty"`
	ListingID string `json:"listing_id,omitempty"`
	Text      string `json:"text,omitempty"`
}

func listLocal(_ context.Context, _ *mcp.CallToolRequest, in listIn) (*mcp.CallToolResult, listOut, error) {
	if in.Title == "" || in.Price <= 0 {
		return nil, listOut{Error: "title and price required"}, nil
	}
	if in.Schema == "" {
		in.Schema = "csv.v1"
	}
	did, seed, err := ownerMaterial()
	if err != nil {
		return nil, listOut{Error: err.Error()}, nil
	}
	bin := os.Getenv("LIST_BIN")
	if bin == "" {
		return nil, listOut{Error: "LIST_BIN not set"}, nil
	}
	cmd := exec.Command(bin, "-title", in.Title, "-schema", in.Schema, "-price", fmt.Sprint(in.Price))
	cmd.Env = append(os.Environ(), "OWNER_DID="+did, "CONTROLLER_SEED="+seed)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return nil, listOut{Error: text}, nil
	}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "LISTING_ID=") {
			id = strings.TrimPrefix(line, "LISTING_ID=")
		}
	}
	return nil, listOut{ListingID: id, Text: text}, nil
}

func ownerMaterial() (string, string, error) {
	did := strings.TrimPrefix(os.Getenv("OWNER_DID"), "0x")
	seed := strings.TrimSpace(os.Getenv("CONTROLLER_SEED"))
	dir := os.Getenv("KEYSTORE_DIR")
	if dir == "" {
		b, err := os.ReadFile(filepath.Join(mustHome(), ".arthneura", "owner.dir"))
		if err == nil {
			dir = strings.TrimSpace(string(b))
		}
	}
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), "agents", "me")
	}
	if seed == "" {
		b, err := os.ReadFile(filepath.Join(dir, "controller.seed"))
		if err != nil {
			return "", "", fmt.Errorf("controller.seed missing: register first")
		}
		seed = strings.TrimSpace(string(b))
	}
	if did == "" {
		b, err := os.ReadFile(filepath.Join(dir, "owner.did"))
		if err != nil {
			return "", "", fmt.Errorf("owner.did missing: register first")
		}
		did = strings.TrimPrefix(strings.TrimSpace(string(b)), "0x")
	}
	if did == "" || seed == "" {
		return "", "", fmt.Errorf("OWNER_DID and CONTROLLER_SEED missing")
	}
	return did, seed, nil
}

func registerLocal(_ context.Context, _ *mcp.CallToolRequest, in registerIn) (*mcp.CallToolResult, registerOut, error) {
	bin := os.Getenv("REGISTER_BIN")
	if bin == "" {
		return nil, registerOut{Error: "REGISTER_BIN not set"}, nil
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = "me"
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"DOOR="+getenv("DOOR", "https://id.arthneura.com"),
		"LABEL="+label,
		"KEY_LABEL="+label,
		"KEYSTORE_DIR="+getenv("KEYSTORE_DIR", os.Getenv("HOME")+"/agents/"+label),
		"KEYSTORE_PASS="+getenv("KEYSTORE_PASS", "dev-passphrase"),
		"OWN_CONTROLLER=1",
	)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return nil, registerOut{Error: text}, nil
	}
	var did, key string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "DID=") {
			did = strings.TrimPrefix(line, "DID=")
		}
		if strings.HasPrefix(line, "KEY=") {
			key = strings.Trim(strings.TrimPrefix(line, "KEY="), `"`)
		}
	}
	_ = os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".arthneura"), 0o700)
	ks := getenv("KEYSTORE_DIR", filepath.Join(os.Getenv("HOME"), "agents", label))
	_ = os.WriteFile(filepath.Join(os.Getenv("HOME"), ".arthneura", "owner.dir"), []byte(ks), 0o600)
	return nil, registerOut{DID: did, Key: key}, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

type offerIn struct {
	ListingID int64 `json:"listing_id" jsonschema:"listing to offer on"`
	Price     int64 `json:"price" jsonschema:"price, must be > 0"`
}

type offerOut struct {
	Error   string `json:"error,omitempty"`
	OfferID string `json:"offer_id,omitempty"`
	Text    string `json:"text,omitempty"`
}

func offerLocal(_ context.Context, _ *mcp.CallToolRequest, in offerIn) (*mcp.CallToolResult, offerOut, error) {
	if in.ListingID <= 0 || in.Price <= 0 {
		return nil, offerOut{Error: "listing_id and price required"}, nil
	}
	did, seed, err := ownerMaterial()
	if err != nil {
		return nil, offerOut{Error: err.Error()}, nil
	}
	bin := os.Getenv("OFFER_BIN")
	if bin == "" {
		return nil, offerOut{Error: "OFFER_BIN not set"}, nil
	}
	cmd := exec.Command(bin, "-listing", fmt.Sprint(in.ListingID), "-price", fmt.Sprint(in.Price))
	cmd.Env = append(os.Environ(), "OWNER_DID="+did, "CONTROLLER_SEED="+seed)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return nil, offerOut{Error: text}, nil
	}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "OFFER_ID=") {
			id = strings.TrimPrefix(line, "OFFER_ID=")
		}
	}
	return nil, offerOut{OfferID: id, Text: text}, nil
}

type ruleIn struct {
	ListingID int64 `json:"listing_id" jsonschema:"listing this floor applies to"`
	Floor     int64 `json:"floor" jsonschema:"reject offers below this price"`
}

type ruleOut struct {
	Error string `json:"error,omitempty"`
	Text  string `json:"text,omitempty"`
}

func ruleLocal(_ context.Context, _ *mcp.CallToolRequest, in ruleIn) (*mcp.CallToolResult, ruleOut, error) {
	if in.ListingID <= 0 || in.Floor <= 0 {
		return nil, ruleOut{Error: "listing_id and floor required"}, nil
	}
	dir := os.Getenv("OWNER_DIR")
	if dir == "" {
		if b, err := os.ReadFile(filepath.Join(mustHome(), ".arthneura", "owner.dir")); err == nil {
			dir = strings.TrimSpace(string(b))
		}
	}
	if dir == "" {
		return nil, ruleOut{Error: "OWNER_DIR not set"}, nil
	}
	bin := os.Getenv("RULE_BIN")
	if bin == "" {
		return nil, ruleOut{Error: "RULE_BIN not set"}, nil
	}
	cmd := exec.Command(bin, "-listing", fmt.Sprint(in.ListingID), "-floor", fmt.Sprint(in.Floor))
	cmd.Env = append(os.Environ(), "OWNER_DIR="+dir)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return nil, ruleOut{Error: text}, nil
	}
	return nil, ruleOut{Text: text}, nil
}

type decideOut struct {
	Error string `json:"error,omitempty"`
	Text  string `json:"text,omitempty"`
}

func decideLocal(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, decideOut, error) {
	dir := os.Getenv("OWNER_DIR")
	if dir == "" {
		if b, err := os.ReadFile(filepath.Join(mustHome(), ".arthneura", "owner.dir")); err == nil {
			dir = strings.TrimSpace(string(b))
		}
	}
	did := os.Getenv("OWNER_DID")
	seed := os.Getenv("CONTROLLER_SEED")
	if dir != "" && (did == "" || seed == "") {
		if b, err := os.ReadFile(filepath.Join(dir, "owner.did")); err == nil {
			did = strings.TrimSpace(string(b))
		}
		if b, err := os.ReadFile(filepath.Join(dir, "controller.seed")); err == nil {
			seed = strings.TrimSpace(string(b))
		}
	}
	bin := os.Getenv("DECIDE_BIN")
	if dir == "" || did == "" || seed == "" || bin == "" {
		return nil, decideOut{Error: "DECIDE_BIN and owner drawer required"}, nil
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "OWNER_DIR="+dir, "OWNER_DID="+did, "CONTROLLER_SEED="+seed)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return nil, decideOut{Error: text}, nil
	}
	return nil, decideOut{Text: text}, nil
}

type acceptIn struct {
	Offer int64 `json:"offer" jsonschema:"offer id to sign"`
}

type acceptOut struct {
	Error string `json:"error,omitempty"`
	Text  string `json:"text,omitempty"`
}

func acceptLocal(_ context.Context, _ *mcp.CallToolRequest, in acceptIn) (*mcp.CallToolResult, acceptOut, error) {
	bin := os.Getenv("ACCEPT_BIN")
	if bin == "" {
		return nil, acceptOut{Error: "ACCEPT_BIN missing"}, nil
	}
	if in.Offer <= 0 {
		return nil, acceptOut{Error: "offer id required"}, nil
	}
	cmd := exec.Command(bin, "-offer", strconv.FormatInt(in.Offer, 10))
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return nil, acceptOut{Error: text}, nil
	}
	return nil, acceptOut{Text: text}, nil
}


type lockIn struct {
    CommitmentID string `json:"commitment_id" jsonschema:"commitment id to acknowledge and lock"`
}

type lockOut struct {
    Error string `json:"error,omitempty"`
    Text  string `json:"text,omitempty"`
}

func lockLocal(_ context.Context, _ *mcp.CallToolRequest, in lockIn) (*mcp.CallToolResult, lockOut, error) {
    bin := os.Getenv("LOCK_BIN")
    if bin == "" {
        return nil, lockOut{Error: "LOCK_BIN missing"}, nil
    }
    if in.CommitmentID == "" {
        return nil, lockOut{Error: "commitment_id required"}, nil
    }
    cmd := exec.Command(bin)
    cmd.Env = append(os.Environ(), "COMMITMENT_ID="+in.CommitmentID)
    out, err := cmd.CombinedOutput()
    text := strings.TrimSpace(string(out))
    if err != nil {
        if text == "" {
            text = err.Error()
        }
        return nil, lockOut{Error: text}, nil
    }
    return nil, lockOut{Text: text}, nil
}

func main() {
	httpAddr := flag.String("http", "", "if set, streamable HTTP listen addr (example :8787)")
	flag.Parse()
	log.SetOutput(os.Stderr)
	s := mcp.NewServer(&mcp.Implementation{Name: "arthneura", Version: "0.1.4"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_health", Description: "Check ArthNeura market API. No keys."}, health)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_stamp", Description: "Read-only: offer ready for register_commitment?"}, stamp)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_list_listings", Description: "List market listings. Public read. No keys."}, listListings)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_list_offers", Description: "List market offers. Public read. No keys."}, listOffers)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_get_commitment", Description: "Get one commitment from the market index."}, getCommitment)
	mcp.AddTool(s, &mcp.Tool{Name: "arthneura_get_agent", Description: "Public agent profile by DID. No keys."}, getAgent)
	if os.Getenv("REGISTER_BIN") != "" {
		mcp.AddTool(s, &mcp.Tool{Name: "arthneura_register", Description: "Create a local key and register a DID. Runs only on this machine."}, registerLocal)

		if os.Getenv("LIST_BIN") != "" {
			if os.Getenv("OFFER_BIN") != "" {
				mcp.AddTool(s, &mcp.Tool{Name: "arthneura_offer", Description: "Sign an offer on this machine and post it. No chain lock."}, offerLocal)
			}
			if os.Getenv("DECIDE_BIN") != "" {
				mcp.AddTool(s, &mcp.Tool{Name: "arthneura_decide", Description: "Apply saved floors. Accept the highest open offer at or above the floor. Seller sign only."}, decideLocal)
				if os.Getenv("ACCEPT_BIN") != "" {
					mcp.AddTool(s, &mcp.Tool{Name: "arthneura_accept", Description: "Sign this offer with the local key. Other side of decide. No escrow."}, acceptLocal)
					if os.Getenv("LOCK_BIN") != "" {
                        mcp.AddTool(s, &mcp.Tool{Name: "arthneura_lock", Description: "Trigger on-chain acknowledge to lock funds for a commitment. Runs only on this machine."}, lockLocal)
                    }
				}
			}
			if os.Getenv("RULE_BIN") != "" {
				mcp.AddTool(s, &mcp.Tool{Name: "arthneura_rule", Description: "Store a per-listing floor. Reject below, pick highest above. Does not accept."}, ruleLocal)
			}
			mcp.AddTool(s, &mcp.Tool{Name: "arthneura_list", Description: "Sign a listing on this machine and post it. No chain lock."}, listLocal)
		}
	}
	if *httpAddr != "" {
		h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
		log.Printf("MCP HTTP %s MARKET_URL=%s", *httpAddr, marketURL())
		if err := http.ListenAndServe(*httpAddr, h); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mustHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return os.Getenv("HOME")
	}
	return h
}
