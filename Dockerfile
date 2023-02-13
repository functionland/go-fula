FROM golang:1.19-bullseye as build

WORKDIR /go/src/go-fula

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/blox ./cmd/blox

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/blox /usr/bin/

ENTRYPOINT ["/usr/bin/blox"]