# Build environment
# -----------------
FROM golang:1.25.0-alpine3.21 AS builder
LABEL stage=builder

RUN apk add --no-cache ca-certificates git tzdata bash openssl

WORKDIR /src

COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download


COPY docs/ docs/
COPY internal/ internal/
COPY main.go main.go

# Get the current commit hash
ARG COMMIT_HASH

# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -a -ldflags "-X main.Build=${COMMIT_HASH}" -o /bin/server ./main.go

# Deployment environment
# ----------------------
FROM golang:1.25.0-alpine3.21

RUN apk add --no-cache git ca-certificates && \
    adduser -D -u 10001 nonroot


ENV TMPDIR=/home/nonroot/smithery/tmp
ENV GOCACHE=$TMPDIR/.cache
ENV GOMODCACHE=$TMPDIR/.modcache

RUN mkdir -p "$TMPDIR" "$TMPDIR/dummy" "$GOCACHE/go-build" "$GOMODCACHE/cache/download" && \
    chown -R nonroot /home/nonroot && \
    chmod -R 1777 "$TMPDIR"

USER nonroot

WORKDIR "$TMPDIR/dummy"
COPY --chown=nonroot:nonroot dummy.go.mod.txt go.mod
RUN go mod tidy


WORKDIR /home/nonroot/smithery
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/server /bin/server
    


ENTRYPOINT ["/bin/server"]