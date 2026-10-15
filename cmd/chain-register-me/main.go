package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

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
    did := strings.TrimPrefix(strings.TrimSpace(string(didB)), "0x")

    core := os.Getenv("ARTHNEURA_CORE")
    if core == "" {
        core = "../arthneura-core"
    }

    payload := os.Getenv("PAYLOAD")
    if payload == "" {
        payload = "auto-register-payload"
    }

    cmd := exec.Command("cargo", "run", "-q", "-p", "offchain-vector-db")
    cmd.Dir = core
    cmd.Env = append(os.Environ(),
        "ACTION=register",
        "CONTROLLER_SEED="+seed,
        "PROVIDER_DID=0x"+did,
        "CONSUMER_DID=0x"+did, // placeholder, real flow needs both
        "PRICE=100",
        "PAYLOAD="+payload,
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("register failed: %v", err)
    }
    fmt.Println("REGISTER done")
}
