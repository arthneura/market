package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
)

func main() {
    log.SetFlags(0)
    to := os.Getenv("TO_DID")
    if to == "" {
        log.Fatal("TO_DID required")
    }
    amount := os.Getenv("AMOUNT")
    if amount == "" {
        amount = "100"
    }
    core := os.Getenv("ARTHNEURA_CORE")
    if core == "" {
        core = "../arthneura-core"
    }
    cmd := exec.Command("cargo", "run", "-q", "-p", "offchain-vector-db")
    cmd.Dir = core
    cmd.Env = append(os.Environ(),
        "ACTION=transfer",
        "FROM=//Alice",
        "TO="+to,
        "AMOUNT="+amount,
    )
    out, err := cmd.CombinedOutput()
    fmt.Print(string(out))
    if err != nil {
        log.Fatalf("transfer failed: %v", err)
    }
    fmt.Println("TRANSFER done")
}
