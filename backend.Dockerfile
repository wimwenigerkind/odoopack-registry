FROM golang:1.26 AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o app

FROM alpine:3.24

RUN apk add --no-cache git ca-certificates

COPY --from=build /app/app /app

ENTRYPOINT ["/app"]