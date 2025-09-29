FROM nixos/nix AS builder
COPY . /app
WORKDIR /app
RUN nix --extra-experimental-features 'nix-command flakes' build .

FROM alpine
EXPOSE 8080
COPY --from=builder /app/result/bin/cors-proxy /usr/local/bin/cors-proxy
USER nobody
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["cors-proxy"]
