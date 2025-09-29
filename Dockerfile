FROM docker.io/archlinux:base
COPY . /app
WORKDIR /app
RUN pacman -Syu --noconfirm && \
    pacman -S --noconfirm go && \
    go build -o /usr/local/bin/cors-proxy .;
EXPOSE 8080
USER nobody
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/cors-proxy"]
