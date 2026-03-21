FROM golang:1.26.1-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    git \
    make \
    build-essential \
    tzdata \
    less \
    vim \
 && rm -rf /var/lib/apt/lists/*

ENV TZ=Asia/Tokyo
ENV CGO_ENABLED=1
ENV GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download || true

COPY . .

CMD ["bash"]