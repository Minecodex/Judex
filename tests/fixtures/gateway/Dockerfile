FROM golang:1.26.0-alpine AS build
WORKDIR /src
COPY tests/fixtures/gateway/main.go .
RUN CGO_ENABLED=0 go build -trimpath -o /gateway main.go
FROM alpine:3.22
USER 10001:10001
COPY --from=build /gateway /gateway
ENV JUDEX_FIXTURE_ADDR=0.0.0.0:8080 JUDEX_FIXTURE_MODE=collaboration
EXPOSE 8080
ENTRYPOINT ["/gateway"]
