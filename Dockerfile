# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /build

# Copy ui-kit submodule (dependency for frontend)
COPY ui-kit ./ui-kit

# Copy frontend source
COPY frontend ./frontend
WORKDIR /build/frontend

# Install dependencies
RUN npm install

# Build the frontend application
RUN npm run build

# Backend build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files from backend
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /build/backend
RUN go mod download

# Copy source code
COPY backend/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crm .

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/backend/crm /app/crm

# Copy frontend dist to pb_public (where the backend expects it)
COPY --from=frontend-builder /build/frontend/dist ./pb_public/

# Copy public assets
COPY frontend/public/ ./pb_public/

# Expose port
EXPOSE 8080

# Run the app
CMD ["/app/crm", "serve", "--http=0.0.0.0:8080"]
