FROM golang:1.26-alpine AS build
WORKDIR /source
COPY tests/fixtures/v2-gateway/main.go main.go
RUN CGO_ENABLED=0 go build -o /gateway main.go
FROM alpine:3.22
RUN adduser -D -u 10001 fixture
COPY --from=build /gateway /gateway
USER 10001
EXPOSE 8080
ENTRYPOINT ["/gateway"]
