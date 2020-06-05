package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defaultDataDir := filepath.Join(home, ".config", "chromium")
	dataDir := flag.String("data-dir", defaultDataDir, "Chrome user data directory")
	executable := flag.String("exec", "chromium", "Chrome executable")
	addr := flag.String("addr", "localhost:8989", "HTTP proxy address, [host]:port")

	flag.Parse()

	br := NewBrowser()
	br.AddExtraArg("--disable-web-security", "")
	br.SetDataDir(*dataDir)
	br.SetExecutable(*executable)

	rx := Handler(context.Background(), br)
	RunProxy(*addr, rx)
}
