FROM debian:13.6-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends privoxy=4.0.0-2 \
 && rm -rf /var/lib/apt/lists/*
COPY config allowlist.action /etc/privoxy/
USER privoxy
CMD ["privoxy", "--no-daemon", "/etc/privoxy/config"]
