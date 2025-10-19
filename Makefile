.PHONY: build dist test bench clean

PLATFORMS = linux darwin
ARCHITECTURES = amd64 arm64

build:
	go build -o bin/ ./cmd/...

dist:
	@for platform in $(PLATFORMS); do \
		for arch in $(ARCHITECTURES); do \
			GOOS=$$platform GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -o dist/thrust-$$platform-$$arch ./cmd/...; \
		done \
	done

test:
	go test ./...

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem -run=^# ./...

clean:
	rm -rf bin dist
