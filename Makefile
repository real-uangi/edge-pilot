PROTO_FILE := internal/shared/grpcapi/agent_control.proto

.PHONY: proto

proto:
	PATH="$(shell go env GOPATH)/bin:$$PATH" protoc \
		--proto_path=. \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		$(PROTO_FILE)
