FROM alpine:3.20

RUN apk add --no-cache zip bash

COPY build.sh /build.sh
RUN chmod +x /build.sh

ENTRYPOINT ["/build.sh"]
