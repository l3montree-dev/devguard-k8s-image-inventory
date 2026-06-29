FROM golang:1.26.4-alpine3.24@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS golang-builder

# set the working directory
WORKDIR /app

RUN apk add --no-cache curl git

RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.71.1

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# build the scanner
RUN CGO_ENABLED=0 go build -buildvcs=false \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${DATE} -X main.BuiltBy=docker" \
    -o devguard-k8s-image-inventory .

FROM ghcr.io/l3montree-dev/static@sha256:41517aab6bbb1dfae9334e7e46f4b83070765471f55102f5dbcd8c720685e588

COPY --from=golang-builder /app/devguard-k8s-image-inventory /usr/local/bin/devguard-k8s-image-inventory
COPY --from=golang-builder /usr/local/bin/trivy /usr/local/bin/trivy

WORKDIR /usr/local/bin

COPY trivy.yaml /usr/local/bin/trivy.yaml

USER 53111
# checkov:skip=CKV_DOCKER_2:No curl or wget available in distroless image to implement HEALTHCHECK
CMD ["/usr/local/bin/devguard-k8s-image-inventory"]