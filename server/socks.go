package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/thifnmi/proxy-socks-server/logger"
	"github.com/thifnmi/proxy-socks-server/server/auth"
	"github.com/thifnmi/proxy-socks-server/server/socks4a"
	"github.com/thifnmi/proxy-socks-server/server/socks5"
	"github.com/thifnmi/proxy-socks-server/utils"
)

type SocksServer struct {
	config      *utils.Config
	authMethods map[uint8]auth.Authenticator
}

// authenticate is used to handle connection authentication
func (s *SocksServer) SocksServerAuthenticate(conn net.Conn, bufConn io.Reader) (*auth.AuthContext, error) {
	// Get the methods
	methods, err := auth.ReadMethods(bufConn)
	if err != nil {
		return nil, fmt.Errorf("Failed to get auth methods: %v", err)
	}

	// Select a usable method
	for _, method := range methods {
		cator, found := s.authMethods[method]
		if found {
			return cator.Authenticate(bufConn, conn)
		}
	}

	// No usable method found
	return nil, noAcceptableAuth(conn)
}

// noAcceptableAuth is used to handle when we have no eligible
// authentication mechanism
func noAcceptableAuth(conn io.Writer) error {
	conn.Write([]byte{auth.SocksVersion5, auth.NoAcceptable})
	return auth.NoSupportedAuth
}

func NewSocksServer(config *utils.Config) *SocksServer {
	if len(config.AuthMethods) == 0 {
		if config.Credentials != nil {
			config.AuthMethods = []auth.Authenticator{&auth.UserPassAuthenticator{config.Credentials}}
		} else {
			config.AuthMethods = []auth.Authenticator{&auth.NoAuthAuthenticator{}}
		}
	}
	if config.Resolv == nil {
		config.Resolv = utils.DefaultResolver{}
	}
	if config.Dial == nil {
		config.Dial = func(_ context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		}
	}
	authMethods := make(map[uint8]auth.Authenticator)

	for _, a := range config.AuthMethods {
		authMethods[a.GetCode()] = a
	}
	return &SocksServer{config: config, authMethods: authMethods}
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
	logger.Infof("Serving on %s", bindAddr)
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
		bufConn := bufio.NewReader(conn)
		remoteAddr, remotePortStr, _ := net.SplitHostPort(conn.RemoteAddr().String())
		logger.Infof("Received connection from %s:%s", remoteAddr, remotePortStr)
		go func() error {
			defer conn.Close()
			var buf [1]byte
			_, err := io.ReadFull(conn, buf[:])
			if err != nil {
				logger.Infof("Read socks version error: %s", err)
				return err
			}
			_, err = s.SocksServerAuthenticate(conn, bufConn)
			if err != nil {
				err = fmt.Errorf("Failed to authenticate: %v", err)
				logger.Infof("[ERR] socks: %v", err)
				return err
			}

			logger.Infof("Authenticated with method %d from host %s:%s", buf[0], remoteAddr, remotePortStr)
			switch buf[0] {
			case auth.SocksVersion4:
				err = socks4a.HandleConnection(conn)
			case auth.SocksVersion5:
				err = socks5.HandleConnection(conn)
			default:
				err = fmt.Errorf("unacceptable socks version -> (%d) <-", buf[0])
			}
			if err != nil {
				logger.Infof("handle socks connection err: %s",err)
				return err
			}
			return nil
		}()
	}
}
