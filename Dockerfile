# Build environment
# -----------------
FROM golang:1.24.1-alpine3.21 AS builder
LABEL stage=builder

RUN RUN apk add --no-cache ca-certificates git tzdata bash openssl

WORKDIR /src

COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY apis/ apis/
COPY docs/ docs/
COPY internal/ internal/
COPY plumbing/ plumbing/
COPY main.go main.go

# Get the current commit hash
ARG COMMIT_HASH

# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -a -ldflags "-X main.Build=${COMMIT_HASH}" -o /bin/server ./main.go && \
    strip /bin/server

# Deployment environment
# ----------------------
FROM golang:1.24.1-alpine3.21

RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 nonroot


ENV TMPDIR=/home/nonroot/smithery/tmp
ENV GOCACHE=/home/nonroot/smithery/tmp/.cache

WORKDIR /home/nonroot/smithery
RUN mkdir -p "$TMPDIR" "$GOCACHE/go-build" && 
    chown -R nonroot /home/nonroot &&
    chmod -R 1777 "$GOCACHE"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/server /bin/server
    
USER nonroot

ENTRYPOINT ["/bin/server"]