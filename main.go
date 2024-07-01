package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/thifnmi/proxy-socks-server/server"
	"github.com/thifnmi/proxy-socks-server/utils"
)

func main() {
	bindAddr := flag.String("addr", "0.0.0.0", "socks server bind address (optional)")
	bindPort := flag.String("port", "1080", "socks server bind port (optional)")
	dnsAddr := flag.String("dns", "", "specify a dns server (ip:port) to be used for resolving domains (optional)")
	flag.Parse()

	var resolver utils.Resolver
	if *dnsAddr == "" {
		resolver = utils.DefaultResolver{}
	} else {
		if _, _, err := net.SplitHostPort(*dnsAddr); err != nil {
			fmt.Println("DNS server should be in this format 'ip:port'")
			return
		}
		log.Printf("DNS server %v\n", *dnsAddr)
		resolver = utils.NewCustomResolver(*dnsAddr)
	}

	config := &utils.Config{
		Resolv: resolver,
	}
	bindListenner := fmt.Sprintf("%s:%s", *bindAddr, *bindPort)

	s := server.NewSocksServer(config)
	err := s.ListenAndServe("tcp", bindListenner)
	if err != nil {
		log.Println(err)
	}
}
