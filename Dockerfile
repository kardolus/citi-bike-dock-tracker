# Multi-stage build for the dockscan ingester. Static binary on distroless.
FROM golang:1.20-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath \
    -ldflags "-s -w" -o /out/dockscan ./cmd/dockscan

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/dockscan /dockscan
EXPOSE 2112
ENTRYPOINT ["/dockscan"]
