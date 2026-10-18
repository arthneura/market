package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
)

func main() {
    log.SetFlags(0)
    to := os.Getenv("TO")
    if to == "" {
        log.Fatal("TO required")
    }
    amount := os.Getenv("AMOUNT")
    if amount == "" {
        amount = "100"
    }
    bin := os.Getenv("VECTOR_BIN")
    if bin == "" {
        bin = "/usr/local/bin/offchain-vector-db"
    }
    cmd := exec.Command(bin)
    cmd.Env = append(os.Environ(),
        "ACTION=transfer",
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
