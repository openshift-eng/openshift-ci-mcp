BINARY := openshift-ci-mcp
ORG := rh-edge-enablement
IMAGE ?= quay.io/$(ORG)/$(BINARY)
VERSION ?= 0.0.0-dev

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build build-all test test-integration lint smoke smoke-container image push clean

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

build-all:
	@for platform in $(PLATFORMS); do \
		os=$${platform%%/*}; arch=$${platform##*/}; \
		echo "Building $$os/$$arch..."; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o bin/$(BINARY)-$$os-$$arch ./cmd/$(BINARY); \
	done

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

image:
	podman build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest -f Containerfile .

push: image
	podman push $(IMAGE):$(VERSION)
	podman push $(IMAGE):latest

smoke: build
	python3 tests/smoke_test.py --binary bin/$(BINARY)

smoke-container:
	python3 tests/smoke_test.py --container $(IMAGE):latest

clean:
	rm -rf bin/
