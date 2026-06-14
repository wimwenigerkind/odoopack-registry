FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/odoopack-registry

FROM alpine:3.24

RUN apk add --no-cache git ca-certificates

COPY --from=build /out/odoopack-registry /usr/local/bin/odoopack-registry

WORKDIR /data

ENTRYPOINT ["odoopack-registry"]
