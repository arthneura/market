package faucet

import (
    "net/http"
    "sync"

    gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
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

    api, err := gsrpc.NewSubstrateAPI("ws://node:9944")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    meta, err := api.RPC.State.GetMetadataLatest()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    key, err := signature.KeyringPairFromSecret("//Alice", 42)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    dest, err := types.NewMultiAddressFromHexAccountID(did)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    c, err := types.NewCall(meta, "Balances.transfer_allow_death", dest, types.NewUCompactFromUInt(100))
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    ext := types.NewExtrinsic(c)
    genesisHash, err := api.RPC.Chain.GetBlockHash(0)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    rv, err := api.RPC.State.GetRuntimeVersionLatest()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    accountKey, err := types.CreateStorageKey(meta, "System", "Account", key.PublicKey)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    var accountInfo types.AccountInfo
    _, err = api.RPC.State.GetStorageLatest(accountKey, &accountInfo)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    nonce := uint64(accountInfo.Nonce)

    o := types.SignatureOptions{
        BlockHash:          genesisHash,
        Era:                types.ExtrinsicEra{IsImmortalEra: true},
        GenesisHash:        genesisHash,
        Nonce:              types.NewUCompactFromUInt(nonce),
        SpecVersion:        rv.SpecVersion,
        Tip:                types.NewUCompactFromUInt(0),
        TransactionVersion: rv.TransactionVersion,
    }

    err = ext.Sign(key, o)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    _, err = api.RPC.Author.SubmitAndWatchExtrinsic(ext)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    funded[did] = true
    w.Write([]byte(`{"ok":true,"amount":100}`))
}
