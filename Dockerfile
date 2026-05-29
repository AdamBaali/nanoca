FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mpc-server ./cmd/mpc-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/mpc-server /usr/local/bin/mpc-server
ENTRYPOINT ["/usr/local/bin/mpc-server"]
