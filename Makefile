GOTOOLCHAIN ?= local
export GOTOOLCHAIN

.PHONY: build test race vet fmt selfcheck clean

build:
	go build -trimpath -o bin/satctl ./cmd/satctl

test:
	go test ./... -count=1

race:
	go test -race ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -l .

selfcheck: build
	./bin/satctl selfcheck

clean:
	rm -rf bin
