# Execute the same server and web bytes produced by release/build.mjs.
FROM alpine:3.22
ARG TARGETARCH
RUN apk add --no-cache ca-certificates && addgroup -g 10001 judex && adduser -D -u 10001 -G judex judex
WORKDIR /app
COPY candidate/judex-server-linux-${TARGETARCH} /app/judex-server
COPY candidate/web/ /app/web/
COPY LICENSE NOTICE /app/
RUN chmod 0755 /app/judex-server
USER 10001:10001
ENV JUDEX_ENV=production JUDEX_HTTP_ADDR=0.0.0.0:8080 JUDEX_WEB_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/judex-server"]
