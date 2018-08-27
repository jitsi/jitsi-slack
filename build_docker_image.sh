#!/bin/bash

set -x

rm ./main
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/api/
docker build -t $1 .
