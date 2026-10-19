# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/indexer ./cmd/indexer
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS indexer
COPY --from=build /out/indexer /indexer
USER nonroot
ENTRYPOINT ["/indexer"]

FROM gcr.io/distroless/base-debian12:nonroot AS api
COPY --from=build /out/api /api
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
