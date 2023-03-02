build:
	go build -v -race ./...

test:
	go test -v -race -parallel 32 ./...

build-cli:
	go build -o bin/waction main.go

format:
	go fmt ./... && find . -type f -name "*.go" | cut -c 3- | xargs -I{} gofumpt -w "{}"