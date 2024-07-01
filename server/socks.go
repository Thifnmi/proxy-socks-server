package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/thifnmi/proxy-socks-server/server/socks4a"
	"github.com/thifnmi/proxy-socks-server/server/socks5"
	"github.com/thifnmi/proxy-socks-server/utils"
)

const (
	socksVersion4 byte = 4
	socksVersion5 byte = 5
)

type SocksServer struct {
	config *utils.Config
}

func NewSocksServer(config *utils.Config) *SocksServer {
	if config == nil {
		config = &utils.Config{}
	}
	if config.Resolv == nil {
		config.Resolv = utils.DefaultResolver{}
	}
	if config.Dial == nil {
		config.Dial = func(_ context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		}
	}
	return &SocksServer{config: config}
}

func (s *SocksServer) ListenAndServe(network, bindAddr string) error {
	// init socks4 and socks5 config
	socks4a.InitConfig(s.config)
	socks5.InitConfig(s.config)
	listener, err := net.Listen(network, bindAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Println("Serving on", bindAddr)
	return s.Serve(listener)
}

func (s *SocksServer) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		remoteAddr, remotePortStr, _ := net.SplitHostPort(conn.RemoteAddr().String())
		log.Printf("Received connection from %s:%s", remoteAddr, remotePortStr)
		go func() {
			defer conn.Close()
			var buf [1]byte
			_, err := io.ReadFull(conn, buf[:])
			if err != nil {
				log.Println(err)
				return
			}
			switch buf[0] {
			case socksVersion4:
				err = socks4a.HandleConnection(conn)
			case socksVersion5:
				err = socks5.HandleConnection(conn)
			default:
				err = fmt.Errorf("unacceptable socks version -> (%d) <-", buf[0])
			}
			if err != nil {
				log.Println(err)
			}
		}()
	}
}