# Multi-stage build → final image is distroless static, ~15 MB.
# The build stage compiles with -trimpath + -ldflags="-s -w" so the
# resulting binary is reproducible (same source = same checksum) and
# stripped of debug symbols (smaller image, marginally faster cold-start).

FROM golang:1.25-alpine AS builder

# Disable cgo so the final binary is fully static and runs on
# distroless static (no glibc, no musl). This is the standard pattern
# for Go services; the only reason to enable cgo would be a C
# dependency, which this service avoids by design.
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src

# Copy go.mod + go.sum first and run `go mod download` so the dependency
# layer caches separately from the source layer. Subsequent builds that
# only change source files won't re-fetch modules — speeds up local +
# CI iteration by 30-60s once the dependency graph is non-trivial.
COPY go.mod go.sum* ./
RUN go mod download

# Now copy the rest. Any source change invalidates only this layer.
COPY . .

# -trimpath: remove the absolute build path from the binary so the
#            same source produces the same checksum on any machine.
# -ldflags -s -w: strip symbol + debug tables (smaller binary, no
#                 effect on perf).
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Distroless static = ~2 MB base + your static binary = ~15 MB total.
# `nonroot` user is uid 65532; safer than root for the runtime.
FROM gcr.io/distroless/static-debian12:nonroot

# Copy only the compiled binary. No source, no Go toolchain in the final
# image — that's the whole point of a multi-stage build.
COPY --from=builder /out/server /server

USER nonroot:nonroot
EXPOSE 3001

# Distroless has no /bin/sh or wget, so a shell-form probe always fails
# (issue #28). The binary self-probes instead: `/server -healthcheck` GETs
# loopback /health and exits 0 (200) / 1 (anything else). Baked here so the
# image carries its own healthcheck wherever it runs — including the ops-only
# prod compose, which inherits this without a per-deploy edit.
HEALTHCHECK --interval=10s --timeout=3s --retries=3 --start-period=5s \
    CMD ["/server", "-healthcheck"]

# Listening on 0.0.0.0:3001 inside the container; Docker compose maps
# this to the host port (configurable via override).
ENTRYPOINT ["/server"]
