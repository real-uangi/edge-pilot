PROTO_FILE := internal/shared/grpcapi/agent_control.proto
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = \
	-s \
	-w \
	-X edge-pilot/internal/shared/buildinfo.Version=$(VERSION) \
	-X edge-pilot/internal/shared/buildinfo.Commit=$(COMMIT) \
	-X edge-pilot/internal/shared/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: proto build build-control-plane build-agent

proto:
	PATH="$(shell go env GOPATH)/bin:$$PATH" protoc \
		--proto_path=. \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		$(PROTO_FILE)

build: build-control-plane build-agent

build-control-plane:
	mkdir -p dist
	go build -ldflags '$(strip $(LDFLAGS))' -o dist/edge-pilot-control ./cmd/control-plane

build-agent:
	mkdir -p dist
	go build -ldflags '$(strip $(LDFLAGS))' -o dist/edge-pilot-agent ./cmd/agent
