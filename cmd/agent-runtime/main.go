package main

import (
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
    log.Println("agent-runtime started")

    for {
        if err := runOnce(); err != nil {
            log.Printf("cycle error: %v", err)
        }
        time.Sleep(45 * time.Second)
    }
}

func runOnce() error {
    dir, did, err := loadDrawer()
    if err != nil {
        return err
    }
    local := strings.TrimPrefix(did, "0x")
    log.Printf("drawer=%s did=%s...", dir, local[:8])

    market := os.Getenv("MARKET_URL")
    if market == "" {
        market = "https://api.arthneura.com"
    }

    // Simple curl for offers (later can use client package)
    out, err := exec.Command("curl", "-s", market+"/v1/offers").Output()
    if err != nil {
        return err
    }
    log.Printf("offers fetched (%d bytes)", len(out))

    // For now just log. Next step: parse and act.
    log.Println("cycle complete")
    return nil
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
    log.Printf("%s: %s", name, strings.TrimSpace(string(out)))
    return err
}
