set dotenv-load := false

default:
    @just --list

fmt:
    find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w

lint:
    go vet ./...

test:
    go test ./...

check: fmt lint test

doctor:
    @mise doctor
    @git status --short
    @command -v go || true
    @command -v mise || true

diff:
    @git diff --stat
    @git diff --color=always | less -R
