package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/elazarl/goproxy"
)

func RunProxy(addr string, in chan *Rx) {
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (request *http.Request, response *http.Response) {
		tx := make(chan *http.Response)
		in <- &Rx{Req: req, Tx: tx}

		resp := <-tx

		if resp == nil {
			body := []byte("internal server error")
			resp = &http.Response{
				Status:        http.StatusText(500),
				StatusCode:    500,
				Proto:         req.Proto,
				ProtoMajor:    req.ProtoMajor,
				ProtoMinor:    req.ProtoMinor,
				Header:        http.Header{},
				Body:          ioutil.NopCloser(bytes.NewBuffer(body)),
				ContentLength: int64(len(body)),
			}
		}

		return nil, resp
	})

	if err := http.ListenAndServe(addr, proxy); err != nil {
		log.Fatalf("[ERR] %v", err)
	}
}
