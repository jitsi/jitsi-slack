#!/usr/bin/env bash
# This file :
# 1.) Checks if there is a main binary present in the root.
# 2.) Builds the binary if it is not present.
# 3.) Builds a docker image . 

# TODO :
# - Extend the script to check image into ECR repo. 


set -o errexit
set -o nounset
set -o pipefail

TAG=v.0.0.1
main() {
    FILE=main
    if [ -f "$FILE" ]; then
        echo "Deleting existing binary"
        rm ./main
        echo "Building Binary"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/api/
        echo "Building Docker Image"
        docker build -t jitsi/slack-integration:$TAG .

    else 
        echo "Building Binary"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/api/
        echo "Building Docker Image"
        docker build -t jitsi/slack-integration:$TAG .
    fi
}       
main $@
