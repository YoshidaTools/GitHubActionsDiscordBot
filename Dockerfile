FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bot .
COPY --from=builder /app/private-key.pem .
CMD ["./bot"]