FROM docker.io/golang:1.24-alpine AS builder

RUN apk add make

ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* Makefile ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN go env -w GOCACHE=/cache/go-build GOMODCACHE=/cache/go-mod
#RUN --mount=type=cache,target=/cache/go-build --mount=type=cache,target=/cache/go-mod make
RUN make

FROM docker.io/alpine:latest

ARG TARGETPLATFORM

COPY --from=builder /src/bin/* /usr/local/bin

RUN case "$TARGETPLATFORM" in \
    linux/arm*) EXTRAPACKAGES=raspberrypi-utils-vcgencmd;; \
  esac; \
  echo "EXTRAPACKAGES=$EXTRAPACKAGES"; \
  apk add curl iproute2 httpie alsa-utils bash $EXTRAPACKAGES

WORKDIR /dancer
