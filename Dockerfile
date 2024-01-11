FROM golang:1.18.10-alpine3.17 as builder

WORKDIR /build

COPY . .

RUN GOOS=linux go build -ldflags '-s -w' -o bin/metrics-server-exporter

FROM alpine:3.18.5

WORKDIR /app

COPY --from=builder /build/bin/metrics-server-exporter /app/metrics-server-exporter

EXPOSE 2112

CMD ["/app/metrics-server-exporter"]