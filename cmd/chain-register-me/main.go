package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

type stamp struct {
    ProviderDid string `json:"provider_did"`
    ConsumerDid string `json:"consumer_did"`
    Price       int64  `json:"price"`
    Schema      string `json:"schema"`
    MerkleRoot  string `json:"merkle_root"`
    TotalChunks int64  `json:"total_chunks"`
}

func main() {
    log.SetFlags(0)
    offerID := os.Getenv("OFFER_ID")
    if offerID == "" {
        log.Fatal("OFFER_ID required")
    }

    dir := os.Getenv("OWNER_DIR")
    if dir == "" {
        home, _ := os.UserHomeDir()
        b, err := os.ReadFile(filepath.Join(home, ".arthneura", "owner.dir"))
        if err != nil {
            log.Fatal("no OWNER_DIR")
        }
        dir = strings.TrimSpace(string(b))
    }
    seedB, err := os.ReadFile(filepath.Join(dir, "controller.seed"))
    if err != nil {
        log.Fatal(err)
    }
    seed := strings.TrimSpace(string(seedB))
    didB, err := os.ReadFile(filepath.Join(dir, "owner.did"))
    if err != nil {
        log.Fatal(err)
    }
    localDid := strings.TrimPrefix(strings.TrimSpace(string(didB)), "0x")

    market := os.Getenv("MARKET_URL")
    if market == "" {
        market = "https://api.arthneura.com"
    }
    market = strings.TrimRight(market, "/")

    resp, err := http.Get(market + "/v1/offers/" + offerID + "/stamp")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != 200 {
        log.Fatalf("stamp failed: %s", string(body))
    }
    var st stamp
    if err := json.Unmarshal(body, &st); err != nil {
        log.Fatal(err)
    }

    provider := strings.TrimPrefix(st.ProviderDid, "0x")
    consumer := strings.TrimPrefix(st.ConsumerDid, "0x")

    if localDid != provider {
        log.Fatalf("local DID %s is not provider %s", localDid, provider)
    }

    home, _ := os.UserHomeDir()
    binPath := filepath.Join(home, ".arthneura", "bin", "offchain-vector-db")
    if _, err := os.Stat(binPath); err != nil {
        log.Fatalf("offchain-vector-db binary not found at %s", binPath)
    }

    cmd := exec.Command(binPath)
    cmd.Env = append(os.Environ(),
        "ACTION=register",
        "CONTROLLER_SEED="+seed,
        "PROVIDER_DID=0x"+provider,
        "CONSUMER_DID=0x"+consumer,
        fmt.Sprintf("PRICE=%d", st.Price),
        "SCHEMA="+st.Schema,
        "MERKLE_ROOT="+st.MerkleRoot,
        fmt.Sprintf("TOTAL_CHUNKS=%d", st.TotalChunks),
        "CHAIN_WS="+os.Getenv("CHAIN_WS"),
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("register failed: %v", err)
    }

    cid := ""
    for _, line := range strings.Split(string(out), "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "commitment_id=") || strings.Contains(line, "commitment_id=") {
            parts := strings.SplitN(line, "commitment_id=", 2)
            if len(parts) == 2 {
                raw := strings.Fields(parts[1])[0]
                cid = strings.TrimPrefix(raw, "0x")
                break
            }
        }
    }
    if cid == "" || len(cid) != 64 {
        log.Fatalf("could not parse valid commitment_id from register output: %q", cid)
    }

    linkBody := map[string]string{"commitment_id": cid}
    linkJSON, _ := json.Marshal(linkBody)
    linkResp, err := http.Post(market+"/v1/offers/"+offerID+"/commitment", "application/json", bytes.NewReader(linkJSON))
    if err != nil {
        log.Fatalf("commitment link failed: %v", err)
    }
    defer linkResp.Body.Close()
    linkOut, _ := io.ReadAll(linkResp.Body)
    if linkResp.StatusCode != 200 && linkResp.StatusCode != 201 {
        log.Fatalf("commitment link failed (%d): %s", linkResp.StatusCode, string(linkOut))
    }

    fmt.Println("REGISTER done")
    fmt.Println("COMMITMENT linked:", cid)
}
