# Local fallback when the Docker Hub build-tool registry is unavailable.
# RUNTIME_BASE must be an inspected existing Judex runtime (same Alpine/CA/user).
# The backend is cross-compiled from the frozen source with the native Go tool.
ARG RUNTIME_BASE=judex/server:local-runtime
FROM ${RUNTIME_BASE}
USER root
RUN rm -rf /app/web /app/judex-server
ARG VERSION
COPY --from=backend /judex-server-linux /app/judex-server
COPY web/dist /app/web
COPY LICENSE NOTICE /app/
RUN test -f /app/web/downloads/manifest.json && grep -Fq "\"version\": \"${VERSION}\"" /app/web/downloads/manifest.json
USER 10001:10001
WORKDIR /app
ENTRYPOINT ["/app/judex-server"]
