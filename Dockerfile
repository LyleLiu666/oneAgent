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
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Use Tsinghua mirror for apt
RUN sed -i 's/archive.ubuntu.com/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list \
    && sed -i 's/security.ubuntu.com/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list \
    && sed -i 's/ports.ubuntu.com/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list

# Install system dependencies and Python
# Install basic system utilities
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    curl \
    wget \
    git \
    gnupg \
    locales \
    zip \
    unzip \
    nano \
    procps \
    jq \
    tree \
    && rm -rf /var/lib/apt/lists/*

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libssl-dev \
    libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# Install application dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    pandoc \
    poppler-utils \
    ffmpeg \
    python3 \
    python3-pip \
    python3-venv \
    wkhtmltopdf \
    texlive-latex-base \
    fonts-noto-cjk \
    fonts-droid-fallback \
    fonts-wqy-zenhei \
    fonts-wqy-microhei \
    && rm -rf /var/lib/apt/lists/*

# Generate locales
RUN locale-gen en_US.UTF-8
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# Create app user and directory
RUN groupadd -r app && useradd -r -g app -d /app -s /bin/bash -m app \
    && mkdir -p /data \
    && chown -R app:app /data

WORKDIR /app

# Install Python libraries
RUN pip3 install -i https://pypi.tuna.tsinghua.edu.cn/simple --no-cache-dir \
    requests \
    pandas \
    numpy \
    pypdf \
    pdfminer.six \
    beautifulsoup4 \
    Pillow \
    opencv-python-headless \
    pydub \
    librosa 

# Copy the binary
COPY --from=backend-builder --chown=app:app /app/server .

ENV BASH_ROOT_DIR=/data/bash-root

USER app

# Expose port
EXPOSE 8080

# Run the application
CMD ["./server"]
