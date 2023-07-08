FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.20 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o purpleair_exporter main.go

FROM --platform=${TARGETPLATFORM:-linux/amd64} scratch
COPY --from=builder /app/greatriverenergy_exporter /greatriverenergy_exporter
EXPOSE 2024
ENTRYPOINT ["/greatriverenergy_exporter"]
