FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
WORKDIR /app/service/gateway
RUN go mod download && CGO_ENABLED=0 go build -ldflags="-s -w" -o gateway .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/service/gateway/gateway /app/gateway
COPY --from=builder /app/service/gateway/trpc_go.docker.yaml /app/trpc_go.yaml
WORKDIR /app
EXPOSE 8003 9000
CMD ["./gateway"]
