# syntax=docker/dockerfile:1

# --- Quasar SPA ---
FROM node:22-alpine AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# --- Go API + embedded UI ---
FROM golang:1.24-alpine AS gobuild
WORKDIR /src
RUN apk add --no-cache git ca-certificates
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN rm -rf internal/webui/dist && mkdir -p internal/webui/dist
COPY --from=ui /src/ui/dist/spa/ ./internal/webui/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/suiyu-agent ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata nodejs npm python3 py3-pip docker-cli && \
    pip3 install uv --break-system-packages
WORKDIR /app
RUN mkdir -p /app/data/uploads
COPY --from=gobuild /out/suiyu-agent .
COPY skills ./skills
ENV SERVER_PORT=8080
ENV SKILLS_DIR=/app/skills
ENV UPLOAD_DIR=/app/data/uploads
EXPOSE 8080
ENTRYPOINT ["/app/suiyu-agent"]
