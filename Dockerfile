FROM golang:1.24.4-bookworm AS build

ARG upx_version=4.2.4

RUN apt-get update && apt-get install -y --no-install-recommends xz-utils && \
  curl -Ls https://github.com/upx/upx/releases/download/v${upx_version}/upx-${upx_version}-amd64_linux.tar.xz -o - | tar xvJf - -C /tmp && \
  cp /tmp/upx-${upx_version}-amd64_linux/upx /usr/local/bin/ && \
  chmod +x /usr/local/bin/upx && \
  apt-get remove -y xz-utils && \
  rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN go generate ./...

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux GOAMD64=v2 go build -trimpath -tags osusergo,netgo -o server -a -ldflags="-s -w -buildid=" -gcflags="all=-m=0 -l=2 -dwarf=false" -installsuffix cgo

RUN upx --ultra-brute -qq server && upx -t server

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/server /server

ENTRYPOINT ["/server"]
