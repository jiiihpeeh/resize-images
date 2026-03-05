APP_NAME=image-resizer

.PHONY: all build test test-integration clean run tidy setup proto deps

all: build

build: tidy
	go build -o $(APP_NAME) .

test:
	@echo "Running unit tests..."
	go test -v ./...

test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./...

tidy:
	go mod tidy

run: build
	./$(APP_NAME)

setup: tidy
	go run main.go -tui

# Install Go plugins for protoc
deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto: deps
	mkdir -p pb
	PATH=$$(go env GOBIN):$$(go env GOPATH)/bin:$$HOME/go/bin:$$PATH protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pb/resizer.proto

clean:
	rm -f $(APP_NAME)
	rm -f cafe_resized.jpg