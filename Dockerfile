FROM golang:1.27-alpine AS build

# Install build dependencies, UPX and the CA bundle copied into the final image
RUN apk add --no-cache curl upx ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go generate ./...

# Build the binary for native architecture
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -tags osusergo,netgo -o server -a -ldflags="-s -w -buildid=" -gcflags="all=-m=0 -l=2 -dwarf=false" -installsuffix cgo

# Compress the binary with UPX
RUN upx --best --lzma /app/server

# Final stage: the static binary plus the CA bundle it needs for HTTPS, nothing else
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compressed application binary
COPY --from=build /app/server /server

# Run as nobody; no passwd file is needed since the app never looks up users
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/server"]
