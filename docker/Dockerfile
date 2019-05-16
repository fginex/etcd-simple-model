# docker build -t nexcpu/etcd .
# ---
FROM golang:latest

# ---
ENV ETCDCTL_API=3
ENV ETCD_VER=v3.3.13

# ---
ENV DOWNLOAD_URL=https://github.com/etcd-io/etcd/releases/download

# ---
RUN mkdir -p /opt/etcd

# ---
# Install tools
RUN apt-get update 
RUN apt-get install -y netcat
RUN apt-get install -y curl

# ---
# Install etcd binary - this script is from their download page: https://github.com/etcd-io/etcd/releases
RUN rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
RUN rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test
RUN curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
RUN tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /opt/etcd --strip-components=1
RUN rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
RUN rm -rf /tmp/etcd-download-test

# ---
WORKDIR /opt/etcd

# ---
LABEL Name=etcd-nodes Version=0.0.1

