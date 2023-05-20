FROM alpine:3.17
COPY portier-cli /usr/bin/portier-cli
ENTRYPOINT ["/usr/bin/portier-cli"]