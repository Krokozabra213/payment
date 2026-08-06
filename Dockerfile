FROM golang:1.25-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /payment ./cmd/payment


FROM alpine:3.23

WORKDIR /app
COPY --from=build /payment .

CMD ["./payment"]