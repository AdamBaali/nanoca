FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mpc-server ./cmd/mpc-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/mpc-server /usr/local/bin/mpc-server

# Bake the throwaway demo CA into the image so the demo deploys with no extra
# config. This is NOT a real secret. For real use, override at runtime with
# CA_CERT_PEM / CA_KEY_PEM (raw PEM) or CA_CERT / CA_KEY (file paths).
COPY deploy/demo-ca/rootCA.crt /etc/nanoca/rootCA.crt
COPY deploy/demo-ca/rootCA.key /etc/nanoca/rootCA.key
ENV CA_CERT=/etc/nanoca/rootCA.crt \
    CA_KEY=/etc/nanoca/rootCA.key

ENTRYPOINT ["/usr/local/bin/mpc-server"]
