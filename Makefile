.PHONY: all build run test clean

BUILD_DIR ?= build
BIN ?= $(BUILD_DIR)/mvplayer
ARGS ?=

all: build

build:
	mkdir -p "$(dir $(BIN))"
	go build -buildvcs=false -o "$(BIN)" ./cmd/mvplayer

run:
	go run ./cmd/mvplayer $(ARGS)

test:
	go test ./...

clean:
	rm -f "$(BIN)"
