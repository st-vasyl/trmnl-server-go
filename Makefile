BINARY := trmnl-server-go
COVERAGE := coverage.out

.PHONY: build test coverage run clean

build:
	go build -o $(BINARY) main.go

test:
	go test -cover -coverprofile=$(COVERAGE) ./...
	@go tool cover -func=$(COVERAGE) | tail -n 1

coverage: test
	go tool cover -html=$(COVERAGE)

run:
	go run main.go

clean:
	rm -f $(BINARY) $(COVERAGE)
