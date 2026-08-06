FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git build-base
WORKDIR /app

COPY migrations /app/migrations

WORKDIR /cmd
RUN git clone https://github.com/pressly/goose.git

WORKDIR /cmd/goose
RUN go build -tags='no_clickhouse no_mysql no_sqlite3 no_ydb no_libsql no_mssql no_vertica no_ydb' -o /goose ./cmd/goose

WORKDIR /app

FROM alpine:3.18
WORKDIR /app

RUN apk add libc6-compat

COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /goose /usr/local/bin/goose

RUN chmod +x /usr/local/bin/goose

ENV GOOSE_DRIVER=postgres
ENV GOOSE_DBSTRING=
ENV GOOSE_MIGRATION_DIR=/app/migrations

CMD ["/usr/local/bin/goose", "up"]