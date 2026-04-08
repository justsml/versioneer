APP      := versioneer
CMD      := ./cmd/versioneer
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  := -trimpath

.PHONY: build clean test lint

build:
	CGO_ENABLED=0 go build -ldflags='$(LDFLAGS)' $(GOFLAGS) -o $(APP) $(CMD)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(APP)
