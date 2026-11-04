FROM debian:12.12-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends privoxy=3.0.34-1 \
 && rm -rf /var/lib/apt/lists/*
COPY config allowlist.action /etc/privoxy/
USER privoxy
CMD ["privoxy", "--no-daemon", "/etc/privoxy/config"]
