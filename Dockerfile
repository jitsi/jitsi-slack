FROM alpine:latest as alpine

RUN apk add -U --no-cache ca-certificates

FROM scratch
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ADD main /
CMD ["/main"]
EXPOSE 8080
