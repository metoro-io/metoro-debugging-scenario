FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o instabook .

FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/instabook .

EXPOSE 8087

CMD ["./instabook"]
