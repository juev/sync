FROM scratch

ARG NAME="sync"
ARG TARGETARCH="amd64"
ARG TARGETOS="linux"

ADD https://github.com/juev/$NAME/releases/latest/download/$NAME-$TARGETOS-$TARGETARCH /usr/local/bin/$NAME

ENTRYPOINT ["/usr/local/bin/sync"]