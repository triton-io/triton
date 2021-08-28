# Build the manager binary
FROM golang:1.16.6 as builder

ARG VERSION=0.0.1
ARG BUILD_NUMBER=0
ARG GIT_COMMIT=0

WORKDIR /triton
ENV GOPROXY=https://goproxy.cn,direct
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd cmd/
COPY apis apis/
COPY pkg pkg/
COPY version version/

# Build
RUN CGO_ENABLED=0 go build -ldflags "-extldflags '-static' -X 'triton/version.Version=${VERSION}' -X 'triton/version.BuildNumber=${BUILD_NUMBER}' -X 'triton/version.GitCommit=${GIT_COMMIT}'" -o triton cmd/triton/main.go

# Use Ubuntu 20.04 LTS as base image to package the manager binary
FROM ubuntu:focal
# This is required by daemon connnecting with cri
RUN ln -s /usr/bin/* /usr/sbin/ && apt-get update -y \
  && apt-get install --no-install-recommends -y ca-certificates \
  && apt-get clean && rm -rf /var/log/*log /var/lib/apt/lists/* /var/log/apt/* /var/lib/dpkg/*-old /var/cache/debconf/*-old

WORKDIR /app
COPY --from=builder /triton/triton .

ENTRYPOINT ["/app/triton"]
