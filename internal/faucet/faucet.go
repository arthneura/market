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

    api, err := gsrpc.NewSubstrateAPI("ws://172.16.0.4:9944")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    meta, err := api.RPC.State.GetMetadataLatest()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    to, err := types.NewMultiAddressFromHexAccountID(did)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    amount := types.NewUCompactFromUInt(1000000000000)
    c, err := types.NewCall(meta, "Balances.transfer_allow_death", to, amount)
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
    key, err := types.CreateStorageKey(meta, "System", "Account", signature.TestKeyringPairAlice.PublicKey)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    var accountInfo types.AccountInfo
    ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
    if err != nil || !ok {
        http.Error(w, "account info", http.StatusInternalServerError)
        return
    }
    nonce := uint32(accountInfo.Nonce)
    o := types.SignatureOptions{
        BlockHash:          genesisHash,
        Era:                types.ExtrinsicEra{IsImmortalEra: true},
        GenesisHash:        genesisHash,
        Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
        SpecVersion:        rv.SpecVersion,
        Tip:                types.NewUCompactFromUInt(0),
        TransactionVersion: rv.TransactionVersion,
    }
    err = ext.Sign(signature.TestKeyringPairAlice, o)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    _, err = api.RPC.Author.SubmitExtrinsic(ext)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    funded[did] = true
    w.Write([]byte(`{"ok":true,"amount":100}`))
}
