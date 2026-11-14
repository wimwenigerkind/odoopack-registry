FROM golang:1.27.3@sha256:e519d3b0a5b285e5aa53ab9eef02bc6bbb9cb04608389b743a136a9ce75f0d1e AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/web ./cmd/web
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# The pinned base lags behind Alpine's security fixes until Dependabot bumps
# the digest; upgrading here keeps the image scan in CI from failing on them.
RUN apk upgrade --no-cache && apk add --no-cache git ca-certificates

COPY --from=build /out/web /usr/local/bin/web
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/migrate /usr/local/bin/migrate

# The binaries run as an unprivileged user on a read-only root filesystem.
# Only two paths are written: os.TempDir() (/tmp) for clones and zips, and
# storage.local.root (./data/storage, so /data/data/storage) for the local
# storage driver. Both need a writable volume; the storage directory is
# created owned by the runtime user so a fresh named volume inherits that.
# git reads no system or global config, so HOME needs no writable home.
RUN install -d -o 65532 -g 65532 /data/data/storage

ENV HOME=/tmp \
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_GLOBAL=/dev/null

WORKDIR /data

USER 65532:65532

CMD ["web"]
