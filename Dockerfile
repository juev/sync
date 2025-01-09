FROM alpine:latest

COPY sync /

ARG USER_UID=1001

RUN set -eux; \
    \
    adduser -D runner -u $USER_UID; \
    chmod +rx /sync; 

USER runner

ENTRYPOINT ["/sync"]