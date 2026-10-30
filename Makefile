GOLANG_VERSION=$(shell go env GOVERSION)

# go tool nm ./anise | grep Commit
override LDFLAGS += -X "github.com/macaroni-os/anise/pkg/config.BuildTime=$(shell date -u '+%Y-%m-%d %H:%M:%S %Z')"
override LDFLAGS += -X "github.com/macaroni-os/anise/pkg/config.BuildCommit=$(shell git rev-parse HEAD)"
override LDFLAGS += -X "github.com/macaroni-os/anise/pkg/config.BuildGoVersion=$(GOLANG_VERSION)"

NAME ?= anise
PACKAGE_NAME ?= $(NAME)
PACKAGE_CONFLICT ?= $(PACKAGE_NAME)-beta
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

GOBIN := $(shell go env GOPATH)/bin/
GINKGO := $(GOBIN)/ginkgo

.PHONY: all
all: deps build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	GO111MODULE=off go get github.com/onsi/ginkgo/v2/ginkgo
	GO111MODULE=off go get github.com/onsi/gomega/...
	$(GINKGO) -race -r -flake-attempts 3 ./...

.PHONY: test-integration
test-integration:
	tests/integration/run.sh

.PHONY: coverage
coverage:
	go test ./... -race -coverprofile=coverage.txt -covermode=atomic

.PHONY: test-coverage
test-coverage:
	scripts/ginkgo.coverage.sh --codecov

.PHONY: help
help:
	# make all => deps test lint build
	# make deps - install all dependencies
	# make test - run project tests
	# make lint - check project code style
	# make build - build project for all supported OSes

.PHONY: clean
clean:
	rm -rf release/
	rm -rf tests/integration/shunit2
	rm -rf tests/integration/bin

.PHONY: deps
deps:
	go env
	# Installing dependencies...
	GO111MODULE=on go install -mod=mod golang.org/x/lint/golint
	GOBIN=$(GOBIN) go install github.com/onsi/ginkgo/v2/ginkgo
	$(GINKGO) version

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'

.PHONY: build-build
build-build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ./anise-build/anise-build ./anise-build

.PHONY: build-small
build-small:
	@$(MAKE) LDFLAGS+="-s -w" build
	upx --brute -1 $(NAME)

.PHONY: build-build-small
build-build-small:
	@$(MAKE) LDFLAGS+="-s -w" build-build
	upx --brute -1 anise-build/$(NAME)-build

.PHONY: image
image:
	docker build --rm -t anise/base .

.PHONY: lint
lint:
	golint ./... | grep -v "be unexported"

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: test-docker
test-docker:
	docker run -v $(ROOT_DIR):/go/src/github.com/macaroni-os/anise \
				--workdir /go/src/github.com/macaroni-os/anise -ti golang:latest \
				bash -c "make test"

.PHONY: goreleaser-snapshot
goreleaser-snapshot:
	rm -rf dist/ || true
	goreleaser release --skip=validate,publish --snapshot --verbose

multiarch-build:
	GOVERSION=$(GOLANG_VERSION) goreleaser build --snapshot --skip=validate --clean

multiarch-build-small:
	@$(MAKE) multiarch-build
	for file in $(ROOT_DIR)/release/**/* ; do \
		upx --best --lzma --brute -1 $${file} ; \
	done

run-tasks: build
	@cd tests/tasks && lxd-compose a anise-ubuntu --destroy
