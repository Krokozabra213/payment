FROM golang:1.25-alpine AS build

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o currency-sync ./cmd/currency-sync


FROM alpine:3.18

RUN echo "https://mirror.yandex.ru/mirrors/alpine/v3.18/main" > /etc/apk/repositories && \
    echo "https://mirror.yandex.ru/mirrors/alpine/v3.18/community" >> /etc/apk/repositories && \
    apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /app/currency-sync .
COPY --from=build /app/configs ./configs

RUN chmod +x /app/currency-sync
CMD ["./currency-sync"]
