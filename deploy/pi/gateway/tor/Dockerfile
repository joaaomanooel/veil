FROM debian:12.12-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends tor=0.4.9.11-0+deb12u1 \
 && rm -rf /var/lib/apt/lists/*
COPY torrc /etc/tor/torrc
USER debian-tor
CMD ["tor", "-f", "/etc/tor/torrc"]
