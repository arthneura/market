package faucet

import (
    "net/http"
    "sync"

    "github.com/centrifuge/go-substrate-rpc-client/v4/signature"
    "github.com/centrifuge/go-substrate-rpc-client/v4/types"
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
    // TODO: actual transfer using go-substrate-rpc-client
    // For now mark as funded
    funded[did] = true
    w.Write([]byte(`{"ok":true,"amount":100}`))
}
