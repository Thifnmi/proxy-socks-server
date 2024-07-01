package socks4a

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/thifnmi/proxy-socks-server/utils"
)

// const socksServerVersion byte = 4

type command byte

const (
	connect command = 1
	bind    command = 2
)

type addrType byte

const (
	ipv4       addrType = 1
	domainname addrType = 3
	ipv6       addrType = 4
)

type resultCode byte

// 90: request granted
// 91: request rejected or failed
// 92: request rejected becasue SOCKS server cannot connect to identd on the client
// 93: request rejected because the client program and identd report different user-ids.
const (
	requestGranted               resultCode = 90
	requestRejectedOrFailed      resultCode = 91
	requestRejectedCannotConnect resultCode = 92
	requestRejectedDiffUserIds   resultCode = 93
)

const timeoutDuration time.Duration = 5 * time.Second

var currConfig *utils.Config

func InitConfig(config *utils.Config) {
	currConfig = config
}

func HandleConnection(conn net.Conn) error {
	c := newClient(conn)
	return c.handle()
}

type client struct {
	conn net.Conn
	req  *request
}

func newClient(conn net.Conn) *client {
	return &client{conn: conn}
}

func (c *client) handle() error {
	req, err := ParseRequest(c.conn)
	if err != nil {
		return err
	}
	c.req = req
	ctx := context.Background()

	if c.req.addressType == domainname {
		resolvedIP, err := currConfig.Resolv.Resolve(ctx, c.req.DestHost)
		if err != nil {
			return err
		}

		if ip4 := net.IP(resolvedIP).To4(); ip4 != nil {
			c.req.addressType = ipv4
			c.req.DestHost = ip4.String()
		} else {
			c.req.addressType = ipv6
			c.req.DestHost = net.IP(resolvedIP).To16().String()
		}
	}

	switch c.req.cmd {
	case connect:
		return c.handleConnectCmd(ctx)
	case bind:
		return c.handleBindCmd(ctx)
	default:
		c.sendFailure(requestRejectedOrFailed)
		return fmt.Errorf("unsupported command -> (%v) <-", c.req.cmd)
	}
}

func (c *client) handleConnectCmd(ctx context.Context) error {
	// serverConn, err := net.DialTimeout("tcp", net.JoinHostPort(c.req.destHost, strconv.Itoa(int(c.req.destPort))), timeoutDuration)
	serverConn, err := currConfig.Dial(ctx, "tcp", net.JoinHostPort(c.req.DestHost, strconv.Itoa(int(c.req.DestPort))))
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return err
	}
	defer serverConn.Close()

	bindAddr, bindPortStr, _ := net.SplitHostPort(serverConn.LocalAddr().String())
	bindPort, _ := strconv.Atoi(bindPortStr)

	rep := &reply{resCode: requestGranted, bindAddr: bindAddr, bindPort: uint16(bindPort)}
	buf, err := rep.marshal()
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return err
	}

	_, err = c.conn.Write(buf)
	if err != nil {
		return fmt.Errorf("could not write reply to the client")
	}

	errc := make(chan error, 2)

	go func() {
		_, err := io.Copy(serverConn, c.conn)
		if err != nil {
			err = fmt.Errorf("could not copy from client to server, %v", err)
		}
		errc <- err
	}()

	go func() {
		_, err := io.Copy(c.conn, serverConn)
		if err != nil {
			err = fmt.Errorf("could not copy from server to client, %v", err)
		}
		errc <- err
	}()

	return <-errc
}

func (c *client) handleBindCmd(ctx context.Context) error {
	listener, err := net.ListenTCP("tcp4", nil)
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return err
	}
	defer listener.Close()

	_, bindPortStr, _ := net.SplitHostPort(listener.Addr().String())
	bindPort, _ := strconv.Atoi(bindPortStr)
	bindAddr, _, _ := net.SplitHostPort(c.conn.LocalAddr().String())

	rep := &reply{resCode: requestGranted, bindAddr: bindAddr, bindPort: uint16(bindPort)}
	buf, err := rep.marshal()
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return err
	}

	// first reply
	_, err = c.conn.Write(buf)
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return fmt.Errorf("could not write first reply to the client")
	}

	listener.SetDeadline(time.Now().Add(timeoutDuration))
	bindConn, err := listener.Accept()
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return err
	}
	defer bindConn.Close()

	connectedIP := bindConn.RemoteAddr().(*net.TCPAddr).IP
	if !net.IP.IsUnspecified(connectedIP) && net.IP.Equal(net.ParseIP(c.req.DestHost), connectedIP) {
		c.sendFailure(requestRejectedOrFailed)
		return fmt.Errorf("bind: mismatch is found")
	}

	// second reply
	_, err = c.conn.Write(buf)
	if err != nil {
		c.sendFailure(requestRejectedOrFailed)
		return fmt.Errorf("could not write second reply to the client")
	}

	errc := make(chan error, 2)

	go func() {
		_, err := io.Copy(c.conn, bindConn)
		if err != nil {
			err = fmt.Errorf("could not copy from server to client, %v", err)
		}
		errc <- err
	}()

	go func() {
		_, err := io.Copy(bindConn, c.conn)
		if err != nil {
			err = fmt.Errorf("could not copy from client to server, %v", err)
		}
		errc <- err
	}()

	return <-errc
}

func (c *client) sendFailure(code resultCode) error {
	rep := &reply{resCode: code, bindAddr: "0.0.0.0", bindPort: 0}
	buf, _ := rep.marshal()
	_, err := c.conn.Write(buf)
	return err
}

// +----+----+----+----+----+----+----+----+----+----+....+----+
// | VN | CD | DSTPORT |      DSTIP        | USERID       |NULL|
// +----+----+----+----+----+----+----+----+----+----+....+----+
//    1    1      2              4           variable       1
type request struct {
	cmd         command
	addressType addrType
	DestHost    string
	DestPort    uint16
}

func ParseRequest(conn net.Conn) (*request, error) {
	var buf [7]byte
	_, err := io.ReadFull(conn, buf[:])
	if err != nil {
		return nil, fmt.Errorf("could not read request header")
	}
	var oneByteBuf [1]byte
	for {
		_, err = io.ReadFull(conn, oneByteBuf[:])
		if err != nil {
			return nil, fmt.Errorf("could not read (a byte) from the userid")
		}
		if oneByteBuf[0] == 0 {
			break
		}
	}
	cmd := command(buf[0])
	destPort := binary.BigEndian.Uint16(buf[1:3])
	var destHost string
	var addressType addrType
	if isDomainUnresolved(buf[3:7]) {
		domainName := make([]byte, 0, 20) // this is an estimate of the domain name length
		for {
			_, err = io.ReadFull(conn, oneByteBuf[:])
			if err != nil {
				return nil, fmt.Errorf("could not read (a byte) from the domain name")
			}
			if oneByteBuf[0] == 0 {
				break
			}
			domainName = append(domainName, oneByteBuf[0])
		}
		destHost = string(domainName)
		addressType = domainname
	} else {
		destHost = net.IP(buf[3:7]).String()
		addressType = ipv4
	}
	return &request{cmd: cmd, addressType: addressType, DestHost: destHost, DestPort: destPort}, nil
}

func isDomainUnresolved(ip []byte) bool {
	return bytes.Equal(ip[:3], []byte{0, 0, 0}) && ip[3] != 0 // IP address 0.0.0.x
}

// +----+----+----+----+----+----+----+----+
// | VN | CD | DSTPORT |      DSTIP        |
// +----+----+----+----+----+----+----+----+
//    1    1      2              4
type reply struct {
	resCode  resultCode
	bindAddr string
	bindPort uint16
}

func (r *reply) marshal() ([]byte, error) {
	buf := make([]byte, 2, 8)
	buf[0] = 0
	buf[1] = byte(r.resCode)
	var bindPortBinary [2]byte
	binary.BigEndian.PutUint16(bindPortBinary[:], r.bindPort)
	bindAddrBinary := net.ParseIP(r.bindAddr).To4()
	if bindAddrBinary == nil {
		return nil, fmt.Errorf("invalid IPv4 address (in reply header)")
	}
	buf = append(buf, bindPortBinary[:]...)
	buf = append(buf, bindAddrBinary...)
	return buf, nil
}
