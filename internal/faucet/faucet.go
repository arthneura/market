package faucet

import (
    "net/http"
    "os"
    "os/exec"
    "sync"
)

var (
    funded = make(map[string]bool)
    mu     sync.Mutex
)

func Handler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method", http.StatusMethodNotAllowed)
        return
    }
    did := r.URL.Query().Get("did")
    if did == "" {
        http.Error(w, "did required", http.StatusBadRequest)
        return
    }
    mu.Lock()
    defer mu.Unlock()
    if funded[did] {
        http.Error(w, "already funded", http.StatusConflict)
        return
    }

    bin := os.Getenv("TRANSFER_BIN")
    if bin == "" {
        bin = "/usr/local/bin/transfer-me"
    }
    cmd := exec.Command(bin)
    cmd.Env = append(os.Environ(),
        "ACTION=transfer",
        "TO="+did,
        "AMOUNT=1000000000000000",
        "ARTHNEURA_CORE=/home/azureuser/arthneura-core",
    )
    out, err := cmd.CombinedOutput()
    if err != nil {
        http.Error(w, string(out), http.StatusInternalServerError)
        return
    }
    funded[did] = true
    w.Write([]byte(`{"ok":true,"amount":1000000000000000}`))
}
