FROM scratch
ADD https://github.com/juev/sync/releases/latest/download/sync-linux-amd64 /sync

ENTRYPOINT ["/sync"]