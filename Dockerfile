# Build stage
FROM golang:1.23 AS buildstage

WORKDIR /go-fula

# Copy go module files and download dependencies
COPY ./go-fula/go.mod ./go-fula/go.sum ./
RUN go mod download -x

# Copy the rest of the application source code
COPY ./go-fula/ .

# Build binaries with CGO disabled for static binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/blox && \
    CGO_ENABLED=0 GOOS=linux go build -o /wap ./wap/cmd && \
    CGO_ENABLED=0 GOOS=linux go build -o /initipfs ./modules/initipfs && \
    CGO_ENABLED=0 GOOS=linux go build -o /initipfscluster ./modules/initipfscluster

# Final stage
FROM alpine:3.20

# Install packages from stable repos (NM 1.46.6 — compatible with host NM 1.46.0)
# IMPORTANT: Do NOT use edge/testing for networkmanager packages — a newer nmcli
# than the host's NM daemon causes "unknown property" errors (e.g. mac-address-denylist)
RUN apk update && \
    apk add --no-cache hostapd iw wireless-tools networkmanager-wifi networkmanager-cli \
        networkmanager-dnsmasq dhcpcd iptables curl && \
    apk add --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing mergerfs

WORKDIR /

# Copy binaries from the build stage
COPY --from=buildstage /app /app
COPY --from=buildstage /wap /wap
COPY --from=buildstage /initipfs /initipfs
COPY --from=buildstage /initipfscluster /initipfscluster

# Copy and set permissions for the startup script
COPY ./go-fula.sh /go-fula.sh
RUN chmod +x /go-fula.sh

# Expose necessary ports
EXPOSE 40001 5001

# Set the entrypoint command
CMD ["/go-fula.sh"]