FROM debian:12.12-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends tor=0.4.9.11-0+deb12u1 \
 && rm -rf /var/lib/apt/lists/*
RUN install -d -o debian-tor -g debian-tor -m 0700 /var/lib/tor-control
COPY torrc /etc/tor/torrc
COPY --chmod=0755 rotate.sh /usr/local/bin/rotate.sh
USER debian-tor
CMD ["tor", "-f", "/etc/tor/torrc"]
