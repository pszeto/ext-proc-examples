FROM golang:1.23-bookworm AS builder
WORKDIR /
COPY go.* ./
RUN go mod download
COPY main.go ./
RUN  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main

FROM alpine
WORKDIR /
COPY --from=builder /main /main
EXPOSE 18080
ENTRYPOINT ["/main"]