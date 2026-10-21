package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
)

func main() {
    log.SetFlags(0)
    offerID := os.Getenv("OFFER_ID")
    if offerID == "" {
        log.Fatal("OFFER_ID required")
    }
    merkle := os.Getenv("MERKLE_ROOT")
    chunksStr := os.Getenv("TOTAL_CHUNKS")
    if merkle == "" || chunksStr == "" {
        log.Fatal("MERKLE_ROOT and TOTAL_CHUNKS required")
    }
    chunks, err := strconv.ParseInt(chunksStr, 10, 64)
    if err != nil {
        log.Fatal("TOTAL_CHUNKS must be integer")
    }

    u := os.Getenv("MARKET_URL")
    if u == "" {
        u = "https://api.arthneura.com"
    }
    u = strings.TrimRight(u, "/")

    payload, _ := json.Marshal(map[string]any{
        "merkle_root":       merkle,
        "total_chunks":      chunks,
        "expires_in_blocks": 1000,
    })

    resp, err := http.Post(u+"/v1/offers/"+offerID+"/spec", "application/json", bytes.NewReader(payload))
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    fmt.Print(string(body))
    if resp.StatusCode != 200 {
        log.Fatalf("spec failed: %d", resp.StatusCode)
    }
    fmt.Println("SPEC done")
}
