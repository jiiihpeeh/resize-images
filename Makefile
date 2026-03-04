APP_NAME=image-resizer

.PHONY: all build test test-integration clean run tidy setup

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

clean:
	rm -f $(APP_NAME)
	rm -f cafe_resized.jpg