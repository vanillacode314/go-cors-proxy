FROM nixos/nix as builder
COPY . /app
WORKDIR /app

RUN nix --extra-experimental-features 'nix-command flakes' build .
EXPOSE 8080
CMD [ "/app/result/bin/cors-proxy" ]
