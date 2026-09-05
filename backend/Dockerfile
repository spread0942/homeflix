# Build
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# Run
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
ENV MEDIA_ROOT=/data
ENV PORT=8080
EXPOSE 8080
VOLUME ["/data"]
CMD ["/app/server"]
