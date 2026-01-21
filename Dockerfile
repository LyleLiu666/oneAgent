# Frontend build stage
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

# Install pnpm
RUN npm install -g pnpm

# Copy package files
COPY frontend/package.json frontend/pnpm-lock.yaml* ./

# Install dependencies
RUN pnpm install --no-frozen-lockfile

# Copy frontend source
COPY frontend/ ./

# Build
RUN pnpm build

# Backend build stage
FROM golang:1.24-alpine AS backend-builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true

# Copy backend source
COPY backend/ ./

# Create static directory and copy frontend build
RUN mkdir -p cmd/server/static
COPY --from=frontend-builder /app/frontend/dist/ ./cmd/server/static/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary
COPY --from=backend-builder /app/server .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./server"]
