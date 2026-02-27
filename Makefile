# Variables
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD | tr '/' '-')
HOSTNAME ?= $(shell hostname -s)
ROOTDIR ?= $(shell git rev-parse --show-toplevel)

# Default target
.PHONY: default
default: precommit

# Run
.PHONY: run
run:
	@go run -mod=mod main.go

# Development workflow
.PHONY: precommit
precommit: ensure generate format test check addlicense
	@echo "ready to commit"

.PHONY: generate
generate:
	find . -name "*_suite_test.go" -not -path './vendor/*' -exec dirname {} \; | sort -u | xargs -n 1 go generate -mod=mod

.PHONY: ensure
ensure:
	go mod tidy
	go mod verify

.PHONY: format
format:
	find . -type f -name 'go.mod' -not -path './vendor/*' -exec go run -mod=mod github.com/shoenig/go-modtool -w fmt "{}" \;
	find . -type f -name '*.go' -not -path './vendor/*' -exec gofmt -w "{}" +
	go run -mod=mod github.com/incu6us/goimports-reviser/v3 -project-name github.com/bborbe/vault-cli -format ./...
	find . -type d -name vendor -prune -o -type f -name '*.go' -print0 | xargs -0 -n 10 go run -mod=mod github.com/segmentio/golines --max-len=100 -w

.PHONY: test
test:
	go test -mod=mod -p=$${GO_TEST_PARALLEL:-1} -cover -race $(shell go list -mod=mod ./... | grep -v /vendor/)

.PHONY: check
check: lint vet errcheck vulncheck osv-scanner gosec trivy

.PHONY: lint
lint:
	go run -mod=mod github.com/golangci/golangci-lint/cmd/golangci-lint run --config .golangci.yml ./...

.PHONY: vet
vet:
	go vet -mod=mod $(shell go list -mod=mod ./... | grep -v /vendor/)

.PHONY: errcheck
errcheck:
	go run -mod=mod github.com/kisielk/errcheck -ignore '(Close|Write|Fprint)' $(shell go list -mod=mod ./... | grep -v /vendor/)

.PHONY: vulncheck
vulncheck:
	go run -mod=mod golang.org/x/vuln/cmd/govulncheck $(shell go list -mod=mod ./... | grep -v /vendor/)

.PHONY: osv-scanner
osv-scanner:
	@if [ -f .osv-scanner.toml ]; then \
		go run -mod=mod github.com/google/osv-scanner/v2/cmd/osv-scanner --config .osv-scanner.toml --recursive .; \
	else \
		go run -mod=mod github.com/google/osv-scanner/v2/cmd/osv-scanner --recursive .; \
	fi

.PHONY: gosec
gosec:
	go run -mod=mod github.com/securego/gosec/v2/cmd/gosec -exclude=G104 ./...

.PHONY: trivy
trivy:
	trivy fs --scanners vuln,secret --quiet --no-progress --disable-telemetry --exit-code 1 .

.PHONY: addlicense
addlicense:
	go run -mod=mod github.com/google/addlicense -c "Benjamin Borbe" -y $$(date +'%Y') -l bsd $$(find . -name "*.go" -not -path './vendor/*')
	go install github.com/bborbe/teamvault-utils/cmd/teamvault-username@latest
	go install github.com/bborbe/teamvault-utils/cmd/teamvault-password@latest
	go install github.com/onsi/ginkgo/v2/ginkgo@v2.25.3
	sudo port install trivy

formatenv:
	cat example.env | sort > c
	mv c example.env

gomodprepare:
	@for dir in $$(find `pwd` -name go.mod -exec dirname "{}" \; | grep -v vendor); do \
		echo "add excludes and replaces for $${dir}"; \
		cd $${dir}; \
		go mod edit -exclude cloud.google.com/go@v0.26.0; \
		go mod edit -exclude github.com/go-logr/glogr@v1.0.0-rc1; \
		go mod edit -exclude github.com/go-logr/glogr@v1.0.0; \
		go mod edit -exclude github.com/go-logr/logr@v1.0.0-rc1; \
		go mod edit -exclude github.com/go-logr/logr@v1.0.0; \
		go mod edit -exclude go.yaml.in/yaml/v3@v3.0.3; \
		go mod edit -exclude go.yaml.in/yaml/v3@v3.0.4; \
		go mod edit -exclude golang.org/x/tools@v0.38.0; \
		go mod edit -exclude k8s.io/api@v0.34.0; \
		go mod edit -exclude k8s.io/api@v0.34.1; \
		go mod edit -exclude k8s.io/api@v0.34.2; \
		go mod edit -exclude k8s.io/apiextensions-apiserver@v0.34.0; \
		go mod edit -exclude k8s.io/apiextensions-apiserver@v0.34.1; \
		go mod edit -exclude k8s.io/apiextensions-apiserver@v0.34.2; \
		go mod edit -exclude k8s.io/apimachinery@v0.34.0; \
		go mod edit -exclude k8s.io/apimachinery@v0.34.1; \
		go mod edit -exclude k8s.io/apimachinery@v0.34.2; \
		go mod edit -exclude k8s.io/client-go@v0.34.0; \
		go mod edit -exclude k8s.io/client-go@v0.34.1; \
		go mod edit -exclude k8s.io/client-go@v0.34.2; \
		go mod edit -exclude k8s.io/code-generator@v0.34.0; \
		go mod edit -exclude k8s.io/code-generator@v0.34.1; \
		go mod edit -exclude k8s.io/code-generator@v0.34.2; \
		go mod edit -replace k8s.io/kube-openapi=k8s.io/kube-openapi@v0.0.0-20250701173324-9bd5c66d9911; \
		go mod edit -exclude sigs.k8s.io/structured-merge-diff/v6@v6.0.0; \
		go mod edit -exclude sigs.k8s.io/structured-merge-diff/v6@v6.1.0; \
		go mod edit -exclude sigs.k8s.io/structured-merge-diff/v6@v6.2.0; \
		go mod edit -exclude sigs.k8s.io/structured-merge-diff/v6@v6.3.0; \
		cd - >/dev/null; \
	done;