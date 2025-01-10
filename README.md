# sync

A small utility for synchronizing saved links from Pocket to Linkding.

It runs in the background and makes a request to Pocket for new data after a specified time (`SCHEDULE_TIME`).
Then, if the data has been received, a sequential request is made in Linkding to add the link to the database.

Necessary data for startup (environment variables):

```plain
POCKET_CONSUMER_KEY
POCKET_ACCESS_TOKEN
LINKDING_ACCESS_TOKEN
LINKDING_URL
```

Optional variables:

```plain
LOG_LEVEL=INFO
SCHEDULE_TIME=30m
```

## Installation

### Binary file

You can use the binary file from the [release](https://github.com/juev/sync/releases) page. And then run:

```sh
export POCKET_CONSUMER_KEY=123
export POCKET_ACCESS_TOKEN=423
export LINKDING_ACCESS_TOKEN=fdsfsdf22
export LINKDING_URL=https://links.example.com
./sync
```

### Docker

Example `docker-compose.yaml`:

```plain
services:

  sync:
    image: ghcr.io/juev/sync
    restart: always
    environment:
      - POCKET_CONSUMER_KEY=123
      - POCKET_ACCESS_TOKEN=423
      - LINKDING_ACCESS_TOKEN=1234
      - LINKDING_URL=https://links.example.com
      - LOG_LEVEL=INFO
      - SCHEDULE_TIME=30m
```

Launch example:

```sh
❯ docker compose up
[+] Running 1/0
 ✔ Container sync-sync-1  Recreated                                                                                                                                                    0.1s
Attaching to sync-1
sync-1  | [2025-01-10 05:47:21] INFO: Starting {}
sync-1  | [2025-01-10 05:47:22] INFO: Processing {"resolved_url":"https://kittenlabs.de/ip-over-toslink/"}
sync-1  | [2025-01-10 05:47:23] INFO: Added {"url":"https://kittenlabs.de/ip-over-toslink/"}
sync-1  | [2025-01-10 05:47:23] INFO: Processing {"resolved_url":"https://simonwillison.net/2025/Jan/10/ai-predictions/"}
sync-1  | [2025-01-10 05:47:24] INFO: Added {"url":"https://simonwillison.net/2025/Jan/10/ai-predictions/"}
^CGracefully stopping... (press Ctrl+C again to force)
[+] Stopping 1/1
 ✔ Container sync-sync-1  Stopped                                                                                                                                                      0.1s
```
