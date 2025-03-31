FROM golang:1.17-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o product-catalog-service .

FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/product-catalog-service .

EXPOSE 8081

CMD ["./product-catalog-service"] 