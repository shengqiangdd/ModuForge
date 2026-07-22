# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/moduforge/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache wget ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./server"]
