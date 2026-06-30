# Stage 1: Build React frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go binary (with embedded frontend)
FROM golang:1.22-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app/backend
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/backend/static ./static
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /simlink ./main.go

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache \
    alsa-utils \
    alsa-lib \
    ca-certificates \
    tzdata
COPY --from=backend /simlink /simlink
EXPOSE 8080
ENTRYPOINT ["/simlink"]
