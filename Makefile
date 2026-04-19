.PHONY: build test clean

build:
	go build -o lh ./cmd/lh

test:
	go test ./...

clean:
	rm -f lh
