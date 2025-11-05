# syntax=docker/dockerfile:1.7
FROM golang:1.24-alpine AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /workspace/kubemin-cli ./cmd

FROM alpine:3.20
RUN addgroup -S kubemin && adduser -S -G kubemin kubemin
USER kubemin
WORKDIR /home/kubemin

COPY --from=builder /workspace/kubemin-cli /usr/local/bin/kubemin-cli

EXPOSE 8080
ENTRYPOINT ["kubemin-cli"]
