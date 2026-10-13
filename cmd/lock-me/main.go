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

    core := os.Getenv("ARTHNEURA_CORE")
    if core == "" {
        core = "../arthneura-core"
    }

    cmd := exec.Command("cargo", "run", "-q", "-p", "offchain-vector-db")
    cmd.Dir = core
    cmd.Env = append(os.Environ(),
        "ACTION=acknowledge",
        "CONTROLLER_SEED="+seed,
        "COMMITMENT_ID="+cid,
        "CONSUMER_DID="+did,
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("acknowledge failed: %v", err)
    }
    fmt.Println("LOCK done")
}
