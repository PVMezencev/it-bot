.POSIX:
.SILENT:
.SUFFIXES:

CWD		?= ./
SHELL = /bin/bash
HOSTNAME != hostname -f
LOG       = @printf '%s [%s] MAKE: %s\n' "$(HOSTNAME)" "$$(date -u '+%F %T %Z')"
GO       ?= go
GOSUBVER != $(GO) version | grep -Eo '\<go[0-9.]+' | head -n1 | cut -d. -f2
APPROOT = ./
GOFLAGS  ?=
GCFLAGS  ?=
LDFLAGS  ?=

.PHONY: build
build:
	$(MAKE) bin

.PHONY: start
start: fmt vet run
	$(LOG) "$@"

.PHONY: run
run:
	$(GO) run ./it-bot.go --cwd=$(CWD)/configs

.PHONY: fmt
fmt:
	$(LOG) "$@"
	$(GO) fmt *.go
	$(GO) fmt ./bot/*.go
	$(GO) fmt ./rabbitmq-client/*.go

.PHONY: vet
vet:
	$(LOG) "$@"
	$(GO) fmt *.go
	$(GO) fmt ./bot/*.go
	$(GO) fmt ./rabbitmq-client/*.go

it-bot:
	CGO_ENABLED=0 \
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -gcflags "$(GCFLAGS)" \
		-o $(APPROOT)/"$@" \
		./"$$(basename "$@").go"
	-chmod a+x $(APPROOT)/"$@"

.PHONY: bin
bin:
	$(LOG) "$@"
	$(MAKE) clean
	$(MAKE) fmt
	$(MAKE) vet
	$(MAKE) it-bot

.PHONY: clean
clean:
	$(LOG) "$@"
	rm -rf -- $(APPROOT)/it-bot -- $(APPROOT)/it-bot.exe


