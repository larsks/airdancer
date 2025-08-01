FROM docker.io/golang:1.24-alpine AS builder

RUN apk add make iproute2 curl httpie alsa-utils bash

ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* Makefile ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN go env -w GOCACHE=/cache/go-build GOMODCACHE=/cache/go-mod
RUN --mount=type=cache,target=/cache/go-build --mount=type=cache,target=/cache/go-mod make

FROM docker.io/alpine:latest

COPY --from=builder /src/bin/* /usr/local/bin

RUN apk add curl
WORKDIR /dancer
