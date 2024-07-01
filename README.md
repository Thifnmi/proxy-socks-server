
# socks server implementation
`socks-server` is a simple socks proxy server implementation that supports ( `socks5` / `socks4` / `socks4a` ) clients.
## Build

```/bin/bash
go build .
```
## Usage
```/bin/bash
./proxy-socks-server -h
```
```/bin/bash
Usage of proxy-socks-server:
  -addr string
        socks server bind address (default "0.0.0.0")
  -port string
        socks server bind port (default "1080")
  -dns string
        specify a dns server (ip:port) to be used for resolving domains (optional)
```
```/bin/bash
./proxy-socks-server -addr 0.0.0.0 -port 1080 -dns 8.8.8.8:53
```

```/bin/bash
2024/07/01 15:32:17 dns server 8.8.8.8:53
2022/07/01 15:32:17 Serving on 0.0.0.0:1080


```
Now the server is ready to accept connections and handle them.
