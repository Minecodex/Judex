FROM alpine:3.22 AS certificates
RUN apk add --no-cache ca-certificates
FROM debian:bookworm-slim
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN sed -i 's|http://deb.debian.org|https://deb.debian.org|g' /etc/apt/sources.list.d/debian.sources && apt-get -o Acquire::Retries=3 update && apt-get install -y --no-install-recommends python3 libreoffice-writer libreoffice-calc libreoffice-impress fonts-noto-cjk fonts-liberation && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m judex
WORKDIR /app
COPY server.py /app/server.py
USER 10001:10001
EXPOSE 9081
ENTRYPOINT ["python3","/app/server.py"]
