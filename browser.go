package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"context"
	"errors"
	"log"

	"github.com/chromedp/chromedp"
	"github.com/go-pkgz/repeater"
)

type Browser struct {
	executable string
	debugPort  int
	dataDir    string
	extraArgs  map[string]string
}

func NewBrowser() *Browser {
	return &Browser{
		executable: "chromium",
		debugPort:  9222,
		dataDir:    "",
		extraArgs:  make(map[string]string),
	}
}

func (b *Browser) SetExecutable(value string) {
	b.executable = value
}

func (b *Browser) SetDataDir(value string) {
	b.dataDir = value
}

func (b *Browser) AddExtraArg(key, value string) {
	b.extraArgs[key] = value
}

func (b *Browser) Run(ctx context.Context, f func(ctx context.Context) error) error {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Add(1)

	go func(done context.CancelFunc) {
		if err := b.exec(ctx); err != nil {
			log.Printf("[ERR] browser exited: %v", err)
		}
		wg.Done()
		done()
	}(cancel)

	debugURL, err := b.debugURL(ctx)
	if err != nil {
		return err
	}

	allocatorCtx, cancel := chromedp.NewRemoteAllocator(ctx, debugURL)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	return f(chromeCtx)
}

func (b *Browser) exec(ctx context.Context) error {
	args := b.prepareArgs()
	log.Printf("[DEBUG] running: %s %s", b.executable, strings.Join(args, " "))

	// nolint:gosec // b.executable is meant to be user-provided
	execCmd := exec.CommandContext(ctx, b.executable, args...)

	if err := execCmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}
		log.Printf("[ERR] browser error: %v", err)
		return err
	}
	return nil
}

func (b *Browser) prepareArgs() []string {
	args := make(map[string]string)
	for key, value := range b.extraArgs {
		args[key] = value
	}

	if b.dataDir != "" {
		args["--user-data-dir"] = b.dataDir
	}
	args["--headless"] = ""
	args["--remote-debugging-port"] = fmt.Sprintf("%d", b.debugPort)

	execArgs := make([]string, 0, 2*len(args))
	for key, value := range args {
		arg := key
		if value != "" {
			arg = fmt.Sprintf("%s=%s", arg, value)
		}
		execArgs = append(execArgs, arg)
	}
	return execArgs
}

func (b *Browser) debugURL(ctx context.Context) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/version", b.debugPort)

	req, err := http.NewRequestWithContext(ctxTimeout, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	var resp *http.Response
	err = repeater.NewDefault(10, time.Second).Do(ctxTimeout, func() (err error) {
		resp, err = http.DefaultClient.Do(req)
		return
	})
	if err != nil {
		return "", fmt.Errorf("could not connect to the browser: %v", err)
	}

	var data struct {
		URL string `json:"webSocketDebuggerUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.URL, nil
}
