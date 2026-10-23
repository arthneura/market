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
    cid := os.Getenv("COMMITMENT_ID")
    if cid == "" {
        log.Fatal("COMMITMENT_ID required")
    }

    dir := os.Getenv("OWNER_DIR")
    if dir == "" {
        home, _ := os.UserHomeDir()
        b, err := os.ReadFile(filepath.Join(home, ".arthneura", "owner.dir"))
        if err != nil {
            log.Fatal("no OWNER_DIR and no ~/.arthneura/owner.dir")
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

    bin := os.Getenv("CHAIN_BIN")
    if bin == "" {
        bin = filepath.Join(os.Getenv("HOME"), ".arthneura", "bin", "offchain-vector-db")
    }

    ws := os.Getenv("CHAIN_WS")
    if ws == "" {
        ws = "ws://20.219.152.30:9944"
    }

    cmd := exec.Command(bin)
    cmd.Env = append(os.Environ(),
        "ACTION=acknowledge",
        "CONTROLLER_SEED="+seed,
        "COMMITMENT_ID="+cid,
        "CONSUMER_DID="+did,
        "CHAIN_WS="+ws,
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("acknowledge failed: %v", err)
    }
    fmt.Println("LOCK done")
}
