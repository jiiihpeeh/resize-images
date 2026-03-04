FROM golang:1.23-bookworm AS builder

RUN apt-get update && apt-get install -y pkg-config libvips-dev gcc

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o image-resizer .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y libvips42 ca-certificates

WORKDIR /app

COPY --from=builder /app/image-resizer .

EXPOSE 8080

CMD ["./image-resizer"]
