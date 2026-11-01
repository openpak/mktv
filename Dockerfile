# Multi-stage, cross-compiled: the build stage runs on the runner's native arch and Go
# emits the target arch, so linux/arm64 builds need no QEMU. No dependencies beyond the
# standard library; templates, the stylesheet and the font are embedded in the binary.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/mktv ./cmd/mktv

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 65532 -h /app app
COPY --from=build /out/mktv /app/mktv
USER 65532:65532
ENV MKTV_ADDR=:20190
EXPOSE 20190
ENTRYPOINT ["/app/mktv"]
