FROM golang:1.26-alpine

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /service ./ads/internal/cmd/ads-api

ENTRYPOINT ["/service"]
