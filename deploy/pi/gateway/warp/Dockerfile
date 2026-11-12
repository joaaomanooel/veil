FROM golang:1.27.3-bookworm AS build
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOTOOLCHAIN=local
WORKDIR /src
RUN git clone --depth 1 --branch v2.3.1 https://github.com/ViRb3/wgcf.git \
 && test "$(git -C wgcf rev-parse HEAD)" = de6bba7147dc965a1b8e4de113a709750697ccea \
 && cd wgcf && go build -o /out/wgcf .
RUN git clone --depth 1 --branch v1.1.3 https://github.com/whyvl/wireproxy.git \
 && test "$(git -C wireproxy rev-parse HEAD)" = 31a9a34498267d7fb8aeff05aba79d929b15eb39 \
 && cd wireproxy && go get golang.org/x/net@v0.62.0 && go mod tidy \
 && go build -o /out/wireproxy ./cmd/wireproxy

FROM debian:13.7-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --home-dir /var/lib/warp warp \
 && install -d -o warp -g warp -m 0700 /var/lib/warp
COPY --from=build /out/wgcf /out/wireproxy /usr/local/bin/
COPY wireproxy.conf /etc/wireproxy.conf
COPY --chmod=0755 entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --chmod=0755 tunnel-check.sh /usr/local/bin/tunnel-check
COPY --chmod=0755 init.sh /usr/local/bin/warp-init
USER warp
WORKDIR /var/lib/warp
CMD ["/usr/local/bin/entrypoint.sh"]
