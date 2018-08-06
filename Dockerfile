FROM golang:1.10.3

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/jitsi/jitsi-slack

# Install godep and vendor dependencies.
RUN go get -u github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/jitsi/jitsi-slack
RUN dep ensure

# Build the api command.
RUN go install github.com/jitsi/jitsi-slack/cmd/api

ENTRYPOINT /go/bin/api

EXPOSE 8080
