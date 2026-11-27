.PHONY: build test spec cli fuzz vet fmt lint vuln check conformance vscode vscode-test sublime-test npm npm-test goreleaser release-snapshot

build:            ## build binary into ./bin
	go build -o bin/concourse-lint ./cmd/concourse-lint

# archtest asks go list for the imports, which the test cache does not see:
# it runs again without the cache, or a new import passes as "cached".
test:             ## all tests
	go test ./...
	go test -count=1 ./internal/archtest

spec:             ## only the specification cases from spec/
	go test ./internal/lint -run TestSpec

cli:              ## only the black-box tests of the CLI
	go test ./cmd/concourse-lint

# CI runs a shorter round: make fuzz FUZZTIME=30s
FUZZTIME ?= 60s

fuzz:             ## 60 seconds of fuzzing
	go test ./internal/lint -run '^$$' -fuzz FuzzCheck -fuzztime $(FUZZTIME)

vet:
	go vet ./...

fmt:
	gofmt -w .

# The tools have their own module in tools/, so that their dependencies stay
# out of the go.mod of the linter.
lint:             ## staticcheck
	go tool -modfile=tools/go.mod staticcheck ./...

# Needs the network: it loads the Go vulnerability database.
vuln:             ## known vulnerabilities in dependencies and the Go standard library
	go tool -modfile=tools/go.mod govulncheck ./...

check: fmt vet lint test   ## before every commit

# A module of its own (conformance/concourse/go.mod), so the Concourse
# dependency stays out of the main module. Needs the network on the first run.
conformance:      ## the field tables against the structs of Concourse (atc)
	cd conformance/concourse && go vet ./... && go test ./...

vscode-test:      ## type check and unit tests of the VS Code extension
	cd editors/vscode && pnpm install --frozen-lockfile && pnpm run check && pnpm test

sublime-test:     ## unit tests of the Sublime Text package (Python 3.8 or newer)
	python3 -m unittest discover -s editors/sublime/tests -t editors/sublime/tests

vscode:           ## VS Code extension packages for all platforms into dist/vscode (Go, Node, pnpm)
	scripts/package-vscode.sh

npm:              ## npm package with the binaries of all platforms into dist/npm (Go, Node)
	scripts/package-npm.sh

npm-test: npm     ## npm package: launcher tests, then npx concourse-lint in an empty project
	scripts/smoke-test-npm.sh

# GoReleaser has a module of its own (tools/release/go.mod): it needs the
# newest Go and brings hundreds of dependencies. It is built once into
# bin/goreleaser, with the toolchain its go.mod asks for (GOTOOLCHAIN=auto);
# the release binaries are then built by the Go of go.mod.
goreleaser:       ## GoReleaser in the version of tools/release/go.mod into bin/
	cd tools/release && GOTOOLCHAIN=auto go build -o ../../bin/goreleaser github.com/goreleaser/goreleaser/v2

# The container image needs Docker with buildx; without Docker it is
# skipped.
release-snapshot: goreleaser  ## release archives and image of the current commit into dist/release, nothing published
	scripts/test-release-version.sh
	scripts/test-release-notes.sh
	bin/goreleaser release --snapshot --clean $(if $(shell command -v docker),,--skip=docker)
	scripts/check-release-archives.sh
	$(if $(shell command -v docker),scripts/smoke-test-docker.sh ghcr.io/mfast99/concourse-lint:snapshot-amd64)
