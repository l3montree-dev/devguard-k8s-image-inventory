FROM golang:1.26.4-alpine3.24@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS golang-builder

# set the working directory
WORKDIR /app

RUN apk add --no-cache curl git

RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.71.1

COPY . .

# build the scanner
RUN CGO_ENABLED=0 go build -buildvcs=false -o devguard-operator .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=golang-builder /app/devguard-operator /usr/local/bin/devguard-operator
COPY --from=golang-builder /usr/local/bin/trivy /usr/local/bin/trivy

WORKDIR /usr/local/bin

COPY trivy.yaml /usr/local/bin/trivy.yaml

CMD ["/usr/local/bin/devguard-operator"]