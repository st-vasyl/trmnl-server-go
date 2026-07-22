# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26.4 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module downloads separately from the source build.
COPY go.mod go.sum ./
RUN go mod download

# modernc.org/sqlite is pure Go, so CGO can stay off -> a fully static binary
# and trivial cross-compilation.
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/trmnl-server .

# Data mountpoint, pre-created so a fresh named volume inherits its ownership.
RUN mkdir -p /out/data

# ---- final ----
FROM alpine:3.24

# ca-certificates: outbound HTTPS to plugin APIs + Google Fonts.
# tzdata: correct timezone handling. busybox wget (already present) drives the
# healthcheck. Run as a non-root user.
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S trmnl \
    && adduser -S -G trmnl -u 10001 trmnl

COPY --from=builder /out/trmnl-server /usr/local/bin/trmnl-server
COPY --from=builder --chown=10001:10001 /out/data /data

# The app writes trmnl.db, public/, fonts/ and icons/ relative to its working
# directory, so /data (a named volume) holds all persistent state.
WORKDIR /data
USER trmnl:trmnl

EXPOSE 8080

ENTRYPOINT ["trmnl-server"]
CMD ["-c", "/config/config.yaml"]
