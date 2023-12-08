FROM golang:1.21 AS builder

WORKDIR /go/src/go-fula

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/blox ./cmd/blox

FROM gcr.io/distroless/static-debian11
COPY --from=builder /go/bin/blox /usr/bin/

ENTRYPOINT ["/usr/bin/blox"]