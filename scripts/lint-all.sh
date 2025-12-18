set -e
set -o pipefail

gofmt -l -w -s ./cmd ./lib
go vet ./cmd/... ./lib/...
which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0
golangci-lint run

