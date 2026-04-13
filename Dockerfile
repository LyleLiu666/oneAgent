# syntax=docker/dockerfile:1.4

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
FROM golang:1.25.3-alpine AS backend-builder

RUN apk add --no-cache git

WORKDIR /app

# Bring external SDK workspaces into the build without copying them into this repo.
COPY --from=agentsdk . /ext/agentsdk
COPY --from=memorysdk . /ext/memorysdk

# Copy go mod files
COPY backend/go.mod backend/go.sum* ./
RUN go mod edit -replace codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git=/ext/agentsdk \
    && go mod edit -replace codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git=/ext/memorysdk \
    && go mod download

# Copy backend source
COPY backend/ ./

# Create static directory and copy frontend build
RUN mkdir -p cmd/server/static
COPY --from=frontend-builder /app/frontend/dist/ ./cmd/server/static/

# Build the application
RUN go mod edit -replace codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git=/ext/agentsdk \
    && go mod edit -replace codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git=/ext/memorysdk \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Final stage
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies and Python
# Install basic system utilities
RUN apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 update \
    && apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 install -y --no-install-recommends \
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
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

# Install build dependencies
RUN apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 update \
    && apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libssl-dev \
    libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# Install application dependencies
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 install -y --no-install-recommends \
    nodejs \
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
RUN pip3 install --no-cache-dir --retries 5 --timeout 120 \
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
