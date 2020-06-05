all:
	go generate -v ./js
	go build -ldflags="-s -w" mkuznets.com/go/chroxy
