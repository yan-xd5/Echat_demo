FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
WORKDIR /app/service/controller
RUN go mod download && CGO_ENABLED=0 go build -ldflags="-s -w" -o controller .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/service/controller/controller /app/controller
COPY --from=builder /app/service/controller/trpc_go.docker.yaml /app/trpc_go.yaml
WORKDIR /app
EXPOSE 8002
CMD ["./controller"]
