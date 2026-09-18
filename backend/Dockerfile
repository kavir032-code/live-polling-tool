# --- Build stage ---
FROM golang:1.22-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# --- Run stage ---
FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/server .

# Render sets PORT for you at runtime; the app already reads it via getEnv("PORT", ...)
EXPOSE 8080
CMD ["./server"]
