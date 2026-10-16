.PHONY: all build run test install uninstall clean

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

install:
	./scripts/install.sh

uninstall:
	./scripts/uninstall.sh

clean:
	rm -f "$(BIN)"
