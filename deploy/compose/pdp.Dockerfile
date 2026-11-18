# Built from the repository root: docker build -f deploy/compose/pdp.Dockerfile .
FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
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
