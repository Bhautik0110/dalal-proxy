### dalal proxy

Basic reverse proxy implementation using round-robin method.

### Usage
```shell
Dalal Proxy
------------
Simple proxy implementation & demonstration <->
Usage:
-workers (default: 1)
==> Specify number of workers (workers:requests)
-hosts (default: "")
==> Specify upstream server | server(s) using comma
-scheme (default: https)
==> Protocol scheme for upstream server
-disable-cache (default: false)
==> Remove Cache-Control header from response
-port (default: 65535)
==> Proxy port number
Example:
dalal-proxy -workers=40 -hosts=example.com -scheme=https -disable-cache=true

# Open localhost:65353
```
