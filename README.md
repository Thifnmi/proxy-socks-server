
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


You can use with [iptables](https://en.wikipedia.org/wiki/Iptables#:~:text=iptables%20is%20a%20user%2Dspace,to%20treat%20network%20traffic%20packets.) to only allow your ip

Command:
```/bin/bash
# List config iptables

iptables -L -v

  Chain INPUT (policy ACCEPT 87 packets, 6571 bytes)
   pkts bytes target     prot opt in     out     source               destination         
1    14   586 ACCEPT     tcp  --  any    any     192.168.21.122       anywhere             tcp dpt:socks
2     0     0 ACCEPT     tcp  --  any    any     192.168.21.101       anywhere             tcp dpt:socks
3    28  2067 DROP       tcp  --  any    any     anywhere             anywhere             tcp dpt:socks
 
  Chain FORWARD (policy DROP 0 packets, 0 bytes)
   pkts bytes target                    prot opt in      out      source               destination         
1 47459   37M DOCKER-USER               all  --  any     any      anywhere             anywhere            
2 47459   37M DOCKER-ISOLATION-STAGE-1  all  --  any     any      anywhere             anywhere            
3 25007   30M ACCEPT                    all  --  any     docker0  anywhere             anywhere             ctstate RELATED,ESTABLISHED
4   161 10280 DOCKER                    all  --  any     docker0  anywhere             anywhere            
5 22291 6774K ACCEPT                    all  --  docker0 !docker0 anywhere             anywhere            
6     0     0 ACCEPT                    all  --  docker0 docker0  anywhere             anywhere            
 
  Chain OUTPUT (policy ACCEPT 69 packets, 3588 bytes)
   pkts bytes target     prot opt in     out     source               destination         
 
  Chain DOCKER (1 references)
   pkts bytes target     prot opt in     out     source               destination         
 
  Chain DOCKER-ISOLATION-STAGE-1 (1 references)
   pkts bytes target                    prot opt in      out       source               destination         
1 22291 6774K DOCKER-ISOLATION-STAGE-2  all  --  docker0 !docker0  anywhere             anywhere            
2 47459   37M RETURN                    all  --  any     any       anywhere             anywhere            
 
  Chain DOCKER-ISOLATION-STAGE-2 (1 references)
   pkts bytes target     prot opt in     out      source               destination         
1     0     0 DROP       all  --  any    docker0  anywhere             anywhere            
2 22291 6774K RETURN     all  --  any    any      anywhere             anywhere            
 
  Chain DOCKER-USER (1 references)
   pkts bytes target     prot opt in     out     source               destination         
1 47459   37M RETURN     all  --  any    any     anywhere             anywhere


# Allow connection from 192.168.22.101 to port 1080
iptables -A INPUT -p tcp -s 192.168.22.101 --dport 1080 -j ACCEPT

# Drop all other connections to port 1080
iptables -A INPUT -p tcp --dport 1080 -j DROP

# Delete a rule iptables
# iptables -D <chain> <rule-number>

iptables -D INPUT <line-of-rule-number-in-chain>
```

If run images builded by Dockerfile with normal command, rule iptables is not working because the hosts of container is different with hosts of VM. To fix it, run image with command:

```/bin/bash
docker run -dp <expose-port>:<container-port> --network host --name <your-container-name> <your-images-id>
```