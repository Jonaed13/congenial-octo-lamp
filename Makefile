.PHONY: build run clean install-playwright

build:
	go build -o orchestrator main.go

run: build
	./orchestrator

install-playwright:
	go run github.com/playwright-community/playwright-go/cmd/playwright install

deps:
	go mod download

clean:
	rm -f orchestrator

all: deps install-playwright build
