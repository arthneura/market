package main

import (
    "bytes"
    "encoding/hex"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "github.com/arthneura/arthneura-market/internal/deliver"
	"github.com/arthneura/arthneura-market/internal/merkle"
)

type stamp struct {
    OfferID         int64  `json:"offer_id"`
    Ready           bool   `json:"ready"`
    Reason          string `json:"reason"`
    MerkleRoot      string `json:"merkle_root"`
    TotalChunks     int64  `json:"total_chunks"`
    CommitmentID    string `json:"commitment_id"`
}

type commitment struct {
    CommitmentID string `json:"commitment_id"`
    MerkleRoot   string `json:"merkle_root"`
    TotalChunks  int64  `json:"total_chunks"`
    DeliverURL   string `json:"deliver_url"`
    Status       string `json:"status"`
}

func main() {
    offer := flag.Int64("offer", 0, "accepted offer id")
    id := flag.String("id", os.Getenv("COMMITMENT_ID"), "commitment id hex")
    flag.Parse()
    market := os.Getenv("MARKET_URL")
    if market == "" {
        market = "http://127.0.0.1:8080"
    }

    var rootHex string
    var total int
    var base string
    var label string

    if *offer > 0 {
        var s stamp
        if err := getJSON(fmt.Sprintf("%s/v1/offers/%d/stamp", market, *offer), &s); err != nil {
            log.Fatal(err)
        }
        if !s.Ready {
            log.Fatalf("stamp not ready: %s", s.Reason)
        }
        if s.CommitmentID == "" {
            log.Fatal("stamp missing commitment_id")
        }
        var c commitment
        if err := getJSON(market+"/v1/commitments/"+s.CommitmentID, &c); err != nil {
            log.Fatal(err)
        }
        if strings.TrimSpace(c.DeliverURL) == "" {
            log.Fatal("no deliver_url — provider must announce first")
        }
        if c.MerkleRoot != s.MerkleRoot || c.TotalChunks != s.TotalChunks {
            log.Fatal("stamp root does not match indexed commitment")
        }
        rootHex = s.MerkleRoot
        total = int(s.TotalChunks)
        base = strings.TrimRight(c.DeliverURL, "/")
        label = fmt.Sprintf("offer=%d commitment=%s", s.OfferID, s.CommitmentID)
    } else {
        *id = strings.TrimSpace(*id)
        if *id == "" {
            log.Fatal("pass -offer N or -id COMMITMENT_HEX")
        }
        var c commitment
        if err := getJSON(market+"/v1/commitments/"+*id, &c); err != nil {
            log.Fatal(err)
        }
        if strings.TrimSpace(c.DeliverURL) == "" {
            log.Fatal("no deliver_url — provider must announce first")
        }
        rootHex = c.MerkleRoot
        total = int(c.TotalChunks)
        base = strings.TrimRight(c.DeliverURL, "/")
        label = "commitment=" + c.CommitmentID
    }

    rootb, err := hex.DecodeString(rootHex)
    if err != nil || len(rootb) != 32 {
        log.Fatal("bad merkle_root on board")
    }
    var root [32]byte
    copy(root[:], rootb)
    if total <= 0 {
        log.Fatal("total_chunks missing")
    }
    log.Printf("%s url=%s total=%d root=%s", label, base, total, rootHex)

    outDir := os.Getenv("EVIDENCE_DIR")
    if outDir == "" {
        cid := strings.TrimPrefix(strings.TrimSpace(os.Getenv("COMMITMENT_ID")), "0x")
        if cid == "" && strings.Contains(label, "commitment=") {
            parts := strings.Split(label, "commitment=")
            cid = strings.TrimSpace(parts[len(parts)-1])
        }
        if cid == "" {
            cid = "unknown"
        }
        home, _ := os.UserHomeDir()
        outDir = filepath.Join(home, ".arthneura", "artifacts", cid)
    }
    if err := os.MkdirAll(outDir, 0o755); err != nil {
        log.Fatal(err)
    }

    var all []byte
    for i := 0; i < total; i++ {
        var box deliver.ChunkBox
        if err := getJSON(fmt.Sprintf("%s/chunks/%d", base, i), &box); err != nil {
            log.Fatal(err)
        }
        if !deliver.VerifyBox(box, total, root) {
            raw, _, _ := deliver.DecodeBox(box)
            h := merkle.HashBytes(raw)
            fmt.Printf("CHUNK_HASH_%d=0x%x\n", i, h)
            log.Fatalf("chunk %d VERIFY FAIL against board root", i)
        }
        raw, _, err := deliver.DecodeBox(box)
        if err != nil {
            log.Fatal(err)
        }
        if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%d.bin", i)), raw, 0o644); err != nil {
            log.Fatal(err)
        }
        h := merkle.HashBytes(raw)
        fmt.Printf("CHUNK_HASH_%d=0x%x\n", i, h)
        all = append(all, raw...)
        log.Printf("chunk %d ok", i)
    }
    received := filepath.Join(outDir, "received.bin")
    if err := os.WriteFile(received, all, 0o644); err != nil {
        log.Fatal(err)
    }
    var lines [][]byte
    for i := 0; i < total; i++ {
        raw, err := os.ReadFile(filepath.Join(outDir, fmt.Sprintf("%d.bin", i)))
        if err != nil {
            log.Fatal(err)
        }
        lines = append(lines, raw)
    }
    csvPath := filepath.Join(outDir, "received.csv")
    if err := os.WriteFile(csvPath, append(bytes.Join(lines, []byte("\n")), '\n'), 0o644); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("RECEIVED_FILE=%s\n", csvPath)
    fmt.Printf("RECEIVED_BIN=%s\n", received)
    fmt.Printf("EVIDENCE_DIR=%s\n", outDir)
    log.Printf("all chunks verified")
}

func getJSON(url string, dest any) error {
    res, err := http.Get(url)
    if err != nil {
        return err
    }
    defer res.Body.Close()
    if res.StatusCode != 200 {
        return fmt.Errorf("%s -> %s", url, res.Status)
    }
    return json.NewDecoder(res.Body).Decode(dest)
}
