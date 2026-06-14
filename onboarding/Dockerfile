# syntax=docker/dockerfile:1.6
#
# Multi-stage build for the onboarding Go service.
# Stage 1 pins the Go version to match .mise.toml / go.mod (1.26.4); keep in
# sync manually or let Renovate bump via the Docker image tag.
#
# Final stage uses distroless nonroot (UID 65532) so the image runs under
# restricted K8s PodSecurity without extra annotations.

# renovate: datasource=docker depName=golang
FROM golang:1.26.4-alpine AS build
WORKDIR /src

# Restore as a distinct layer so go.mod/go.sum changes invalidate less
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/onboarding ./

# renovate: datasource=docker depName=gcr.io/distroless/static-debian12
FROM gcr.io/distroless/static-debian12:nonroot
# Container-internal HTTP port. The binary reads APP_PORT (defaults to 8080);
# ENV provides the in-image default, EXPOSE documents it. Override at run time
# with `-e APP_PORT=...` (compose wires it from APP_INTERNAL_PORT).
ARG APP_INTERNAL_PORT=8080
ENV APP_PORT=${APP_INTERNAL_PORT}
COPY --from=build /out/onboarding /onboarding
EXPOSE ${APP_INTERNAL_PORT}
USER 65532:65532
ENTRYPOINT ["/onboarding"]
