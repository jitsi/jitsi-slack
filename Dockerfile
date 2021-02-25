FROM alpine:latest as alpine

RUN apk add -U --no-cache bash ca-certificates jq

ADD main /
COPY build/run.sh /

CMD ["/run.sh"]
EXPOSE 8080
