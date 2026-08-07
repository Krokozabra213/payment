FROM golang:1.25-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /payment ./cmd/payment


FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY russian_trusted_root_ca.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

WORKDIR /app
COPY --from=build /payment .

CMD ["./payment"]