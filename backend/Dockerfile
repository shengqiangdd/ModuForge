# Frontend build stage
FROM node:22-alpine AS frontend-builder

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --include=dev

COPY frontend/ .
RUN npm run build

# Backend build stage
FROM golang:1.25-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/moduforge/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache wget ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /app/server .
COPY --from=frontend-builder /app/dist /app/dist
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./server"]
