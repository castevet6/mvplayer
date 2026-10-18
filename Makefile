.PHONY: all build release run test test-pr-workflow install uninstall clean

BUILD_DIR ?= build
BIN ?= $(BUILD_DIR)/mvplayer
ARGS ?=
VERSION ?= $(shell cat VERSION)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
RELEASE ?= $(shell test -z "$$(git status --porcelain 2>/dev/null)" && test "$$(git describe --tags --exact-match HEAD 2>/dev/null)" = "v$(VERSION)" && printf true || printf false)
VERSION_FLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.release=$(RELEASE)

all: build

build:
	mkdir -p "$(dir $(BIN))"
	go build -buildvcs=false -ldflags '$(VERSION_FLAGS)' -o "$(BIN)" ./cmd/mvplayer

release:
	mkdir -p "$(dir $(BIN))"
	go build -buildvcs=false -ldflags '-s -w $(VERSION_FLAGS)' -o "$(BIN)" ./cmd/mvplayer

run:
	go run -ldflags '$(VERSION_FLAGS)' ./cmd/mvplayer $(ARGS)

test:
	go test ./...

test-pr-workflow:
	bash -n scripts/pr.sh scripts/review-pr.sh scripts/test-pr-workflow.sh
	./scripts/test-pr-workflow.sh

install:
	./scripts/install.sh

uninstall:
	./scripts/uninstall.sh

clean:
	rm -f "$(BIN)"
