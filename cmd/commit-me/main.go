package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
)

func main() {
    log.SetFlags(0)
    offerID := os.Getenv("OFFER_ID")
    cid := os.Getenv("COMMITMENT_ID")
    if offerID == "" || cid == "" {
        log.Fatal("OFFER_ID and COMMITMENT_ID required")
    }

    u := os.Getenv("MARKET_URL")
    if u == "" {
        u = "https://api.arthneura.com"
    }
    u = strings.TrimRight(u, "/")

    payload, _ := json.Marshal(map[string]any{
        "commitment_id": cid,
    })

    resp, err := http.Post(u+"/v1/offers/"+offerID+"/commitment", "application/json", bytes.NewReader(payload))
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    fmt.Print(string(body))
    if resp.StatusCode != 200 {
        log.Fatalf("commit failed: %d", resp.StatusCode)
    }
    fmt.Println("COMMIT done")
}
