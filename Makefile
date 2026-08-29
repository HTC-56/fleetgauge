# Makefile — build, test, and package fleetgauge.
#
# `make verify` is the gate a phase must pass. All recipe lines below are
# guarded by the four-command gate: gofmt -l ., go vet ./..., go test ./...,
# bash scripts/scrub-check.sh.

.PHONY: build test vet fmt verify demo dist clean

build:
	CGO_ENABLED=0 go build -trimpath -o fleetgauge ./cmd/fleetgauge

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

verify:
	bash verify.sh

demo:
	go run ./cmd/fleetgauge -demo

dist:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/fleetgauge-linux-amd64 ./cmd/fleetgauge
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/fleetgauge-linux-arm64 ./cmd/fleetgauge

clean:
	rm -f fleetgauge
	rm -rf dist/
