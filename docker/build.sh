#!/bin/sh

docker container stop etcd1 etcd2 etcd3
docker container rm etcd1 etcd2 etcd3
docker image rm nexcpu/etcd

docker build -t nexcpu/etcd .

docker-compose -f docker-compose.yml up
