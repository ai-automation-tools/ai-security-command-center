# Built from the repository root: docker build -f deploy/compose/pdp.Dockerfile .
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY src ./src
RUN CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /ascc-pdp ./src/cmd/ascc-pdp

# Nothing but the binary: no shell, no package manager, not root.
FROM scratch
COPY --from=build /ascc-pdp /ascc-pdp
USER 65532:65532
ENTRYPOINT ["/ascc-pdp"]
