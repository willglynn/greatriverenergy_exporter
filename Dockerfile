FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.20-alpine as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o greatriverenergy_exporter main.go

FROM --platform=${TARGETPLATFORM:-linux/amd64} scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/greatriverenergy_exporter /greatriverenergy_exporter
EXPOSE 2024
ENTRYPOINT ["/greatriverenergy_exporter"]
