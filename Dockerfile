FROM alpine:3.18
COPY portier-cli /usr/bin/portier-cli
ENTRYPOINT ["/usr/bin/portier-cli"]