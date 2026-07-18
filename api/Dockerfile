# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
