FROM golang:1.23 AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/aisphere-auth ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/aisphere-auth /aisphere-auth
USER nonroot:nonroot
EXPOSE 18080
ENTRYPOINT ["/aisphere-auth"]
