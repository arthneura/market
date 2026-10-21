package main

import (
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

    core := os.Getenv("ARTHNEURA_CORE")
    if core == "" {
        core = "../arthneura-core"
    }

    cmd := exec.Command("cargo", "run", "-q", "-p", "offchain-vector-db")
    cmd.Dir = core
    cmd.Env = append(os.Environ(),
        "ACTION=register",
        "CONTROLLER_SEED="+seed,
        "PROVIDER_DID=0x"+provider,
        "CONSUMER_DID=0x"+consumer,
        fmt.Sprintf("PRICE=%d", st.Price),
        "SCHEMA="+st.Schema,
        "MERKLE_ROOT="+st.MerkleRoot,
        fmt.Sprintf("TOTAL_CHUNKS=%d", st.TotalChunks),
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("register failed: %v", err)
    }
    fmt.Println("REGISTER done")
}
