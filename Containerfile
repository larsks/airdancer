FROM docker.io/golang:1.23 AS builder

ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* Makefile ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN make

FROM docker.io/alpine:latest

COPY --from=builder /src/bin/* /usr/local/bin

RUN apk add curl
WORKDIR /dancer
