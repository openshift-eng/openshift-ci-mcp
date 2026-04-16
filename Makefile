BINARY := openshift-ci-mcp
USER := rh_ee_jeroche
IMAGE ?= quay.io/$(USER)/$(BINARY)
VERSION ?= 0.1.0

.PHONY: build test test-integration lint smoke smoke-container image push clean

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

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
