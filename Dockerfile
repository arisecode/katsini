FROM golang:1.25.1-alpine AS build

# Install build dependencies and UPX
RUN apk add --no-cache curl upx

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go generate ./...

# Build the binary for native architecture
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -tags osusergo,netgo -o server -a -ldflags="-s -w -buildid=" -gcflags="all=-m=0 -l=2 -dwarf=false" -installsuffix cgo

# Compress the binary with UPX
RUN upx --best --lzma /app/server

# Final Alpine-based stage
FROM alpine:latest

# Install Chromium and required packages
RUN apk add --no-cache \
    chromium \
    chromium-chromedriver \
    ca-certificates \
    xvfb \
    xauth \
    font-noto-emoji \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

# Copy the compressed application binary
COPY --from=build /app/server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
