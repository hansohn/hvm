ARG GO_VERSION=1.24

# base stage with dependencies
FROM golang:${GO_VERSION}-alpine AS base
RUN apk add --no-cache \
  ca-certificates \
  git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

# linting stage
FROM base AS lint
ARG GOLANGCI_VERSION=v1.64.8
RUN apk add --no-cache curl \
  && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin ${GOLANGCI_VERSION}
COPY . .
RUN golangci-lint run --timeout=5m

# test stage
FROM base AS test
ARG GOTESTSUM_VERSION=v1.12.1
RUN go install gotest.tools/gotestsum@${GOTESTSUM_VERSION}
COPY . .
RUN gotestsum --format testname -- -race -coverprofile=coverage.out -covermode=atomic ./...
RUN go tool cover -func=coverage.out

# security analysis stage
FROM base AS security
ARG GOSEC_VERSION=v2.22.4
RUN go install github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}
COPY . .
RUN gosec ./...

# builder stage
FROM base AS builder
ARG GOOS=linux
ARG GOARCH=amd64
ARG VERSION=dev
COPY . .
RUN CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
  -ldflags="-X main.version=${VERSION} -extldflags=-static" \
  -o hvm .

# export stage - single binary for local extraction via --output
FROM scratch AS export
COPY --from=builder /build/hvm /hvm

# main
FROM alpine:3.21 AS main
RUN apk add --no-cache \
  ca-certificates
WORKDIR /app
COPY --from=builder /build/hvm .
RUN addgroup -g 1000 appuser \
  && adduser -D -u 1000 -G appuser appuser \
  && chown -R appuser:appuser /app
USER appuser
ENV TERM=xterm-256color
ENTRYPOINT ["/app/hvm"]
