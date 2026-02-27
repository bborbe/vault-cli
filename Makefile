include Makefile.variables
include Makefile.precommit
include Makefile.docker
include example.env

SERVICE = bborbe/go-skeleton

run:
	@go run -mod=mod main.go \
	-sentry-dsn="$(shell teamvault-url --teamvault-config ~/.teamvault.json --teamvault-key=${SENTRY_DSN_KEY})" \
	-listen="localhost:${SKELETON_PORT}" \
	-kafka-brokers="${KAFKA_BROKERS}" \
	-datadir="data" \
	-batch-size="100" \
	-v=2

deps:
	go install github.com/bborbe/teamvault-utils/cmd/teamvault-config-parser@latest
	go install github.com/bborbe/teamvault-utils/cmd/teamvault-file@latest
	go install github.com/bborbe/teamvault-utils/cmd/teamvault-url@latest
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