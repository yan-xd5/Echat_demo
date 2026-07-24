FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
WORKDIR /app/service/api
RUN go mod download && CGO_ENABLED=0 go build -ldflags="-s -w" -o api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/service/api/api /app/api
COPY --from=builder /app/service/api/trpc_go.docker.yaml /app/trpc_go.yaml
WORKDIR /app
EXPOSE 8001 9001
CMD ["./api"]
