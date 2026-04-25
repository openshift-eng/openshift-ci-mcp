FROM golang:1.25 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /openshift-ci-mcp ./cmd/openshift-ci-mcp

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /openshift-ci-mcp /openshift-ci-mcp
ENTRYPOINT ["/openshift-ci-mcp"]
