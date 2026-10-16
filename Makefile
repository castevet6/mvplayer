.PHONY: all build run test clean

BIN ?= mvplayer
ARGS ?=

all: build

build:
	go build -buildvcs=false -o $(BIN) ./cmd/mvplayer

run:
	go run ./cmd/mvplayer $(ARGS)

test:
	go test ./...

clean:
	rm -f $(BIN)
