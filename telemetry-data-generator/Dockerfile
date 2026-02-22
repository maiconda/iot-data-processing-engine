FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY main.go .

RUN go build -ldflags="-s -w" -o main main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/main .

CMD ["./main"]