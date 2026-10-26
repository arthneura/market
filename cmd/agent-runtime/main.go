package main

import (
	"os/signal"
	"syscall"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

func main() {
    log.SetFlags(0)
    if len(os.Args) > 1 && os.Args[1] == "status" {
        fmt.Println("agent-runtime: running (check logs for details)")
        return
    }
    log.Println("[INFO] agent-runtime started")

    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

    for {
        select {
        case <-sig:
            log.Println("[INFO] shutting down")
            return
        default:
            if err := runOnce(); err != nil {
                log.Printf("[ERROR] cycle: %v", err)
            }
            time.Sleep(45 * time.Second)
        }
    }
}

type offer struct {
    ID     int64  `json:"id"`
    Status string `json:"status"`
}

type offersResp struct {
    Offers []offer `json:"offers"`
}

func runOnce() error {
    dir, did, err := loadDrawer()
    if err != nil {
        return err
    }
    local := strings.TrimPrefix(did, "0x")
    log.Printf("[INFO] drawer=%s did=%s...", dir, local[:8])

    market := os.Getenv("MARKET_URL")
    if market == "" {
        market = "https://api.arthneura.com"
    }

    // Auto-decide for seller if rules.json exists
    rulesPath := filepath.Join(dir, "rules.json")
    if _, err := os.Stat(rulesPath); err == nil {
        log.Println("[INFO] AUTO-DECIDE")
        seed := strings.TrimSpace(string(mustRead(filepath.Join(dir, "controller.seed"))))
        _ = runBin("decide-me", "OWNER_DIR="+dir, "OWNER_DID="+did, "CONTROLLER_SEED="+seed, "MARKET_URL="+market)
    }

    chainWS := os.Getenv("CHAIN_WS")
    if chainWS == "" {
        chainWS = "ws://20.219.152.30:9944"
    }

    out, err := exec.Command("curl", "-s", market+"/v1/offers").Output()
    if err != nil {
        return err
    }
    var resp offersResp
    if err := json.Unmarshal(out, &resp); err != nil {
        return err
    }

    for _, o := range resp.Offers {
        if o.Status != "accepted" {
            continue
        }
        log.Printf("[INFO] checking offer %d", o.ID)
        stampOut, err := exec.Command("curl", "-s", fmt.Sprintf("%s/v1/offers/%d/stamp", market, o.ID)).Output()
        if err != nil {
            log.Printf("[WARN] stamp %d: %v", o.ID, err)
            continue
        }
        if strings.Contains(string(stampOut), "delivery spec missing") {
            log.Printf("[INFO] AUTO-SPEC offer=%d", o.ID)
            merkle := "9142ff269362d78a4ed27fae5a2a0a28bc9f9f278508139f45ccad8bb0ef326f"
            retry(func() error {
                return runBin("spec-me", "OFFER_ID="+fmt.Sprint(o.ID), "MERKLE_ROOT="+merkle, "TOTAL_CHUNKS=1", "MARKET_URL="+market)
            })
            continue
        }
        var st struct {
            ProviderDID  string `json:"provider_did"`
            ConsumerDID  string `json:"consumer_did"`
            CommitmentID string `json:"commitment_id"`
            Ready        bool   `json:"ready"`
            Reason       string `json:"reason"`
            MerkleRoot   string `json:"merkle_root"`
        }
        if err := json.Unmarshal(stampOut, &st); err != nil {
            continue
        }
        prov := strings.TrimPrefix(st.ProviderDID, "0x")
        cons := strings.TrimPrefix(st.ConsumerDID, "0x")
        if local == prov && strings.Contains(st.Reason, "delivery spec missing") {
            log.Printf("[INFO] AUTO-SPEC offer=%d", o.ID)
            merkle := "9142ff269362d78a4ed27fae5a2a0a28bc9f9f278508139f45ccad8bb0ef326f"
            retry(func() error {
                return runBin("spec-me", "OFFER_ID="+fmt.Sprint(o.ID), "MERKLE_ROOT="+merkle, "TOTAL_CHUNKS=1", "MARKET_URL="+market)
            })
            continue
        }
        if local == prov && (st.CommitmentID == "" || st.CommitmentID == "null") {
            log.Printf("[INFO] AUTO-REGISTER offer=%d", o.ID)
            retry(func() error {
                return runBin("chain-register-me", "OWNER_DIR="+dir, fmt.Sprintf("OFFER_ID=%d", o.ID), "MARKET_URL="+market, "CHAIN_WS="+chainWS)
            })
        }
        if local == cons && st.Ready && st.CommitmentID != "" && st.CommitmentID != "null" {
            log.Printf("[INFO] AUTO-LOCK offer=%d", o.ID)
            retry(func() error {
                return runBin("lock-me", "OWNER_DIR="+dir, "COMMITMENT_ID="+strings.TrimPrefix(st.CommitmentID, "0x"), "CHAIN_WS="+chainWS)
            })
        }
    }
    log.Println("[INFO] cycle complete")
    return nil
}

func retry(fn func() error) {
    for i := 0; i < 3; i++ {
        if err := fn(); err == nil {
            return
        }
        log.Printf("[WARN] retry %d/3", i+1)
        time.Sleep(3 * time.Second)
    }
    log.Println("[ERROR] failed after retries")
}

func loadDrawer() (string, string, error) {
    dir := os.Getenv("OWNER_DIR")
    if dir == "" {
        home, _ := os.UserHomeDir()
        b, err := os.ReadFile(filepath.Join(home, ".arthneura", "owner.dir"))
        if err != nil {
            return "", "", fmt.Errorf("no drawer")
        }
        dir = strings.TrimSpace(string(b))
    }
    didB, err := os.ReadFile(filepath.Join(dir, "owner.did"))
    if err != nil {
        return "", "", err
    }
    return dir, strings.TrimSpace(string(didB)), nil
}

func runBin(name string, env ...string) error {
    bin := filepath.Join(os.Getenv("HOME"), ".arthneura", "bin", name)
    if _, err := os.Stat(bin); err != nil {
        return fmt.Errorf("%s missing", name)
    }
    cmd := exec.Command(bin)
    cmd.Env = append(os.Environ(), env...)
    out, err := cmd.CombinedOutput()
    log.Printf("[INFO] %s: %s", name, strings.TrimSpace(string(out)))
    return err
}

func mustRead(path string) []byte {
    b, err := os.ReadFile(path)
    if err != nil {
        return []byte{}
    }
    return b
}
