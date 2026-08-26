FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/web ./cmd/web
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.24

RUN apk add --no-cache git ca-certificates

COPY --from=build /out/web /usr/local/bin/web
COPY --from=build /out/worker /usr/local/bin/worker

WORKDIR /data

CMD ["web"]
