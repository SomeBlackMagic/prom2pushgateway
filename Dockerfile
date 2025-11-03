# syntax=docker/dockerfile:1.8
# check=error=true

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ENV CGO_ENABLED=0 \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build \
    GOTOOLCHAIN=local \
    TZ=UTC \
    SOURCE_DATE_EPOCH=0

WORKDIR /workspace

## warm up module cache
#COPY go.mod go.sum ./
#RUN \
#    --mount=type=cache,target=/go/pkg/mod \
#    --mount=type=cache,target=/root/.cache/go-build \
#    go mod download

# copy sources
COPY . .


# target parameters for cross-compilation
ARG TARGETOS

ARG BUILDTIME=""
ARG VERSION="dev"
ARG REVISION="000000000000000000000000000000"

# build the binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-$(go env GOOS)} \
    GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build \
      -v \
      -o /workspace/prom2pushgateway \
      -trimpath \
      -mod=readonly \
      -buildvcs=false \
      -tags netgo,osusergo,timetzdata \
      -pgo=auto \
      -ldflags "-s -w -buildid= \
                -extldflags '-static' \
                -X 'main.version=${VERSION}' \
                -X 'main.revision=${REVISION}'" \
      .

# minimal runtime image
FROM busybox

COPY --from=curlimages/curl:8.7.1 /usr/bin/curl /usr/bin/curl

# copy CA certificates for HTTPS support
# (taken from the same alpine version)
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# copy the binary (read/execute permissions are enough)
COPY --from=build --chmod=0555 /workspace/prom2pushgateway /usr/local/bin/prom2pushgateway

# run as non-root (65532 = nobody in most base images)
USER 65532:65532

CMD ["/usr/local/bin/prom2pushgateway"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=1s --retries=3 \
  CMD curl -f http://localhost:8081/healthz || exit 1
