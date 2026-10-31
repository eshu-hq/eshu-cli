# eshu-cli image (Refs eshu-hq/eshu-cli#9).
#
# Multi-stage build: compile the static eshu binary on the pinned Go
# toolchain, then ship only the binary on distroless/static. CGO is off
# because the CLI needs no cgo on linux; the macOS-only install paths
# (pkgutil) compile out via build tags and stay inert here.
#
#   docker build -t eshu-cli:dev .
#   docker run --rm eshu-cli:dev --help

ARG GO_VERSION=1.26.10

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/eshu ./cmd/eshu

FROM gcr.io/distroless/static@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=build /out/eshu /eshu
ENTRYPOINT ["/eshu"]
