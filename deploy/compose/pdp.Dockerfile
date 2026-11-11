# Built from the repository root: docker build -f deploy/compose/pdp.Dockerfile .
FROM golang:1.26.11-alpine@sha256:ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb AS build
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
