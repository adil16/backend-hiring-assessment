FROM golang:1.26-alpine

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown

RUN BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 go build \
      -ldflags="-X github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/version.Version=${VERSION} \
        -X github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/version.GitCommit=${GIT_COMMIT} \
        -X github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/version.BuildDate=${BUILD_DATE}" \
      -o /service \
      ./ads/internal/cmd/ads-api

ENTRYPOINT ["/service"]
