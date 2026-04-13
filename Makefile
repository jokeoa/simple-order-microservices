.PHONY: proto proto-tools test

MODULE := github.com/jokeoa/simple-order-microservices
PROTO_DIR := ../contracts
PROTO_FILES := $(PROTO_DIR)/payment/v1/payment.proto $(PROTO_DIR)/order/v1/order.proto

proto-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	PATH="$$(go env GOPATH)/bin:$$PATH" protoc -I $(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=$(MODULE) \
		--go-grpc_out=. \
		--go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)

test:
	go test ./...
