#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

main() {
    rm -f ./main
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/api/
    docker build -t jitsi/jitsi-slack:$TAG .
}

main $@
