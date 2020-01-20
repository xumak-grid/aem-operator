FROM alpine:3.6
LABEL maintainer="jhernandez@xumak.com"

RUN apk add --no-cache --update ca-certificates

COPY bin/operator /usr/local/bin
CMD ["operator"]
