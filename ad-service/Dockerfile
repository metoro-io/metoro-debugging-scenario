FROM golang:1.17-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ad-service .

FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/ad-service .

EXPOSE 8083

CMD ["./ad-service"] 