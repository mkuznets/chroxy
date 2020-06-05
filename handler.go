package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/rakyll/statik/fs"

	_ "mkuznets.com/go/chroxy/js"
)

type Resp struct {
	Status  int
	Body    []byte `json:"body"`
	Headers map[string]string
}

type Rx struct {
	Req *http.Request
	Tx  chan *http.Response
}

func (r *Rx) Handle(ctx context.Context) {
	defer close(r.Tx)

	log.Printf("[INFO] -> %s %s", r.Req.Method, r.Req.URL.String())

	script, err := requestScript(r.Req)
	if err != nil {
		log.Printf("[ERR] %v", err)
		return
	}

	eval := runtime.Evaluate(script)
	eval.ReturnByValue = true
	eval.ReplMode = true

	result, exc, err := eval.Do(ctx)
	if exc != nil {
		log.Printf("[ERR] %v", errFromExc(exc))
		return
	}
	if err != nil {
		log.Printf("[ERR] %v", err)
	}

	var res Resp
	if err := json.Unmarshal(result.Value, &res); err != nil {
		log.Printf("[ERR] could not decode result: %v", err)
		return
	}

	log.Printf("[INFO] <- %d %dB", res.Status, len(res.Body))

	r.Tx <- makeHTTPResponse(r.Req, &res)
}

func Handler(ctx context.Context, br *Browser) chan *Rx {
	jsFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	libScript, err := fs.ReadFile(jsFS, "/lib.js")
	if err != nil {
		log.Fatal(err)
	}

	rx := make(chan *Rx)

	go func() {
		err := br.Run(ctx, func(ctx context.Context) error {
			if err := chromedp.Run(ctx, runtime.Enable()); err != nil {
				return err
			}

			runCtx := chromedp.FromContext(ctx)
			execCtx := cdp.WithExecutor(ctx, runCtx.Target)

			eval := runtime.Evaluate(string(libScript))
			eval.ReplMode = true

			_, exc, err := eval.Do(execCtx)
			if exc != nil {
				return fmt.Errorf("could not execute lib.js: %v", errFromExc(exc))
			}
			if err != nil {
				return fmt.Errorf("could not execute lib.js: %v", err)
			}

			for {
				select {
				case r := <-rx:
					r.Handle(execCtx)

				case <-ctx.Done():
					if err := chromedp.Cancel(execCtx); err != nil {
						log.Printf("[WARN] %v", err)
					}
					return nil
				}
			}
		})
		if err != nil {
			log.Fatal(err)
		}
	}()

	return rx
}

func errFromExc(exc *runtime.ExceptionDetails) error {
	buf := bytes.NewBuffer(nil)

	enc := json.NewEncoder(buf)
	enc.SetIndent("  ", "")
	if err := enc.Encode(exc); err != nil {
		return fmt.Errorf("could not encode exception: %v", err)
	}

	return fmt.Errorf("browser exception:\n%v", buf.String())
}

func requestScript(req *http.Request) (string, error) {
	format := `await make_request("%s", "%s", %s, "%s");`

	data, err := ioutil.ReadAll(req.Body)
	// noinspection GoUnhandledErrorResult
	defer req.Body.Close()
	if err != nil {
		return "", err
	}

	headers, err := json.Marshal(req.Header)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(format,
		req.Method,
		req.URL.String(),
		string(headers),
		base64.StdEncoding.EncodeToString(data),
	), nil
}

func makeHTTPResponse(req *http.Request, resp *Resp) *http.Response {
	hh := http.Header{}
	for key, value := range resp.Headers {
		hh.Add(key, value)
	}
	hh.Del("content-encoding")

	return &http.Response{
		Status:           http.StatusText(resp.Status),
		StatusCode:       resp.Status,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           hh,
		Body:             ioutil.NopCloser(bytes.NewReader(resp.Body)),
		ContentLength:    int64(len(resp.Body)),
		TransferEncoding: nil,
		Uncompressed:     true,
		Request:          req,
		TLS:              req.TLS,
	}
}
