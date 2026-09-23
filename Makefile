BINARY  := git-add
PKG     := ./cmd/git-add
VERSION := $(shell grep 'const version' cmd/git-add/main.go | sed 's/.*"\(.*\)".*/\1/')
LDFLAGS := -s -w
DIST    := dist

.PHONY: all build install test vet fmt clean release

all: build

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(PKG)

install:
	go install -ldflags="$(LDFLAGS)" $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)

# Static binaries for every supported platform. CGO is off so the keyring
# and certificate code paths use the pure-Go implementations and the result
# runs on a bare system with no shared libraries.
release: clean
	@mkdir -p $(DIST)
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags="$(LDFLAGS)" -o $$out $(PKG) || exit 1; \
	done
	@ls -lh $(DIST)
