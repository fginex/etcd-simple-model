#!/bin/sh
# ---

docker container stop etcd1 etcd2 etcd3
docker container rm etcd1 etcd2 etcd3
docker image rm nexcpu/etcd

docker build -t nexcpu/etcd .

docker-compose -f docker-compose.yml up

# ---
# To test once up and running: 
# 1) Without TLS
#          Execute the follwoing outside docker containers:
#          ./etcdctl endpoint status
#
# 2) With Auto TLS
#          Execute the follwoing inside one of the docker containers (ie etcd1):
#          ./etcdctl --cert=etcd1.etcd/fixtures/client/cert.pem --key=etcd1.etcd/fixtures/client/key.pem --insecure-skip-tls-verify endpoint status
# ---