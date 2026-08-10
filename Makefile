VERSION ?= dev
BINARY := bin/mgl03-homekit-bridge

.PHONY: test build-mgl03 clean

test:
	go test ./...

build-mgl03:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/mgl03-homekit-bridge

clean:
	rm -rf bin
