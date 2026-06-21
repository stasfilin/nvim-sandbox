FROM golang:1.26.4-bookworm

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    bash \
    build-essential \
    ca-certificates \
    curl \
    fd-find \
    git \
    make \
    python3 \
    python3-pip \
    ripgrep \
    shellcheck \
    tmux \
    unzip \
  && rm -rf /var/lib/apt/lists/* \
  && ln -sf /usr/bin/fdfind /usr/local/bin/fd

COPY go.mod go.sum /tmp/nvim-sandbox-modules/
RUN cd /tmp/nvim-sandbox-modules && go mod download

WORKDIR /workspace

CMD ["bash"]
