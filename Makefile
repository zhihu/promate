BIN_PACKAGES = ../cmd/matecarbon ../cmd/mateinsert ../cmd/matequery

build:
	mkdir -p bin && cd bin && for pkg in $(BIN_PACKAGES); do GOOS=linux GOARCH=amd64 go build $$pkg; done

lint:
	golangci-lint run ./...

test: lint
	go test -coverprofile=coverage.out ./...

coverage: lint
	gocov test ./... | gocov-html > coverage.html && open coverage.html
