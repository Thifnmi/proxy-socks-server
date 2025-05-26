package socks5

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/thifnmi/proxy-socks-server/utils"
)

const socksServerVersion byte = 5

// o  X'00' NO AUTHENTICATION REQUIRED
// o  X'01' GSSAPI
// o  X'02' USERNAME/PASSWORD
// o  X'03' to X'7F' IANA ASSIGNED
// o  X'80' to X'FE' RESERVED FOR PRIVATE METHODS
// o  X'FF' NO ACCEPTABLE METHODS
const (
	noAuthMethodRequired byte = 0x00
	noAcceptableMethod   byte = 0xFF
)

type command byte

const (
	connect      command = 1
	bind         command = 2
	udpAssociate command = 3
)

type addrType byte

const (
	ipv4       addrType = 1
	domainname addrType = 3
	ipv6       addrType = 4
	bufio      addrType = 187
)

type resultCode byte

const (
	succeeded               resultCode = 0
	generalSocksFailure     resultCode = 1
	connectionNotAllowed    resultCode = 2
	networkUnreachable      resultCode = 3
	hostUnreachable         resultCode = 4
	connectionRefused       resultCode = 5
	ttlExpired              resultCode = 6
	commandNotSupported     resultCode = 7
	addressTypeNotSupported resultCode = 8
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
	// err := handleHandshake(c.conn)
	// if err != nil {
	// 	c.conn.Write([]byte{socksServerVersion, noAcceptableMethod})
	// 	return err
	// }
	// _, err = c.conn.Write([]byte{socksServerVersion, noAuthMethodRequired})
	// if err != nil {
	// 	return fmt.Errorf("could not reply to the handshake")
	// }
	req, err := ParseRequest(c.conn)
	if err != nil {
		c.sendFailure(generalSocksFailure)
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
	case udpAssociate:
		return c.handleUDPAssociateCmd(ctx)
	default:
		c.sendFailure(commandNotSupported)
		return fmt.Errorf("invalid command -> (%v) <-", c.req.cmd)
	}
}

func (c *client) handleConnectCmd(ctx context.Context) error {
	// serverConn, err := net.DialTimeout("tcp", net.JoinHostPort(c.req.destHost, strconv.Itoa(int(c.req.destPort))), timeoutDuration)
	serverConn, err := currConfig.Dial(ctx, "tcp", net.JoinHostPort(c.req.DestHost, strconv.Itoa(int(c.req.DestPort))))
	if err != nil {
		c.sendFailure(generalSocksFailure)
		return err
	}
	defer serverConn.Close()

	bindAddr, bindPortStr, _ := net.SplitHostPort(serverConn.LocalAddr().String())
	var addressType addrType
	if ip := net.ParseIP(bindAddr); ip != nil {
		if ip.To4() != nil {
			addressType = ipv4
		} else {
			addressType = ipv6
		}
	} else {
		addressType = domainname
	}
	bindPort, _ := strconv.Atoi(bindPortStr)

	rep := &reply{resCode: succeeded, addressType: addressType, bindAddr: bindAddr, bindPort: uint16(bindPort)}
	buf, err := rep.marshal()
	if err != nil {
		c.sendFailure(generalSocksFailure)
		return err
	}

	_, err = c.conn.Write(buf)
	if err != nil {
		return err
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
	// TODO: support bind command
	c.sendFailure(commandNotSupported)
	return fmt.Errorf("[socks5] bind cmd is not supported")
}

func (c *client) handleUDPAssociateCmd(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", "")
	if err != nil {
		c.sendFailure(generalSocksFailure)
		return err
	}
	// TODO: support IPv6
	udpRelaySrv, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {

		c.sendFailure(generalSocksFailure)
		return err
	}
	defer udpRelaySrv.Close()

	bindAddr, bindPortStr, _ := net.SplitHostPort(udpRelaySrv.LocalAddr().String())
	bindPort, _ := strconv.Atoi(bindPortStr)
	rep := &reply{resCode: succeeded, addressType: ipv4, bindAddr: bindAddr, bindPort: uint16(bindPort)}
	replyBuf, err := rep.marshal()
	if err != nil {
		c.sendFailure(generalSocksFailure)
		return err
	}
	_, err = c.conn.Write(replyBuf)
	if err != nil {
		return err
	}

	go func() {
		var buf [1]byte
		for {
			_, err := c.conn.Read(buf[:])
			if err != nil {
				udpRelaySrv.Close()
				break
			}
		}
	}()

	const maxBufSize = math.MaxUint16 - 28 // 28 = [20-byte IP header] + [8-byte UDP header]
	var buf [maxBufSize]byte
	firstReceive := true
	associatedAddr := c.conn.RemoteAddr().(*net.TCPAddr)
	var associatedUDPAddr *net.UDPAddr

	for {
		n, senderAddr, err := udpRelaySrv.ReadFromUDP(buf[:])
		if err != nil {
			return err
		}

		if firstReceive {
			if !net.IP.Equal(senderAddr.IP, associatedAddr.IP) {
				continue
			}
			firstReceive = false
			associatedUDPAddr = senderAddr
		}

		if net.IP.Equal(senderAddr.IP, associatedAddr.IP) {
			req, err := parseUDPAssociateRequest(buf[:n])
			if err != nil {
				return err
			}
			_, err = udpRelaySrv.WriteToUDP(buf[req.payloadIndex:n], req.destAddr)
			if err != nil {
				return err
			}
		} else {
			packet, err := udpAssociateReply(senderAddr, buf[:n])
			if err != nil {
				return err
			}
			_, err = udpRelaySrv.WriteToUDP(packet, associatedUDPAddr)
			if err != nil {
				return err
			}
		}
	}
}

type udpAssociateRequest struct {
	fragmentNumber byte
	addressType    addrType
	destAddr       *net.UDPAddr
	payloadIndex   int
}

func parseUDPAssociateRequest(b []byte) (*udpAssociateRequest, error) {
	fragmentNumber := b[2]
	addressType := addrType(b[3])
	var payloadIndex int
	var host string
	switch addressType {
	case ipv4:
		host = net.IP(b[4:8]).String()
		payloadIndex = 10
	case domainname:
		length := int(b[4])
		host = string(b[5 : length+5])
		payloadIndex = length + 7
	case ipv6:
		host = net.IP(b[4:20]).String()
		payloadIndex = 22
	default:
		return nil, fmt.Errorf("invalid address type code -> (%v) <-", addressType)
	}
	portIndex := payloadIndex - 2
	port := binary.BigEndian.Uint16(b[portIndex : portIndex+2])
	destAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(int(port))))
	if err != nil {
		return nil, fmt.Errorf("invalid destination in the udp associate request")
	}
	return &udpAssociateRequest{fragmentNumber: fragmentNumber, addressType: addressType, destAddr: destAddr, payloadIndex: payloadIndex}, nil
}

func udpAssociateReply(addr *net.UDPAddr, payload []byte) ([]byte, error) {
	var addrLength int
	var addressType addrType
	if ip := addr.IP.To4(); ip != nil {
		addrLength = 4
		addressType = ipv4
	} else {
		addrLength = 16
		addressType = ipv6
	}
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], uint16(addr.Port))
	packet := make([]byte, 0, 4+addrLength+2+len(payload))
	packet = append(packet, 0, 0, 0, byte(addressType))
	packet = append(packet, addr.IP...)
	packet = append(packet, port[:]...)
	packet = append(packet, payload...)
	return packet, nil
}

func (c *client) sendFailure(code resultCode) error {
	// rep := &reply{resCode: code}
	rep := &reply{resCode: code, addressType: ipv4, bindAddr: "0.0.0.0", bindPort: 0}
	buf, _ := rep.marshal()
	_, err := c.conn.Write(buf)
	return err
}

func handleHandshake(conn net.Conn) error {
	var nAuthMethods [1]byte
	_, err := io.ReadFull(conn, nAuthMethods[:])
	if err != nil {
		return fmt.Errorf("could not read len of methods (handshake)")
	}
	authMethods := make([]byte, nAuthMethods[0])
	_, err = io.ReadFull(conn, authMethods)
	if err != nil {
		return fmt.Errorf("could not read the list of auth methods (handshake)")
	}
	for _, method := range authMethods {
		if method == noAuthMethodRequired {
			return nil
		}
	}
	return fmt.Errorf("no auth method is accepted")
}

// +----+-----+-------+------+----------+----------+
// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
type request struct {
	cmd         command
	addressType addrType
	DestHost    string
	DestPort    uint16
}

func ParseRequest(conn net.Conn) (*request, error) {
	var buf [4]byte
	_, err := io.ReadFull(conn, buf[:])
	if err != nil {
		return nil, fmt.Errorf("could not read request header")
	}
	cmd := command(buf[1])
	addressType := addrType(buf[3])
	var destHost string
	switch addressType {
	case ipv4:
		var addr [4]byte
		_, err = io.ReadFull(conn, addr[:])
		if err != nil {
			return nil, fmt.Errorf("could not read dest IPv4 address (in request header)")
		}
		destHost = net.IP(addr[:]).String()
	case domainname:
		var length [1]byte
		_, err = io.ReadFull(conn, length[:])
		if err != nil {
			return nil, fmt.Errorf("could not read dest domain name length (in request header)")
		}
		buf := make([]byte, length[0])
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			return nil, fmt.Errorf("could not read the dest domain name (in request header)")
		}
		destHost = string(buf)
	case ipv6:
		var addr [16]byte
		_, err = io.ReadFull(conn, addr[:])
		if err != nil {
			return nil, fmt.Errorf("could not read dest IPv6 address (in request header)")
		}
		destHost = net.IP(addr[:]).String()
	default:
		return nil, fmt.Errorf("invalid address type code -> (%v) <-", addressType)
	}
	var portBuf [2]byte
	_, err = io.ReadFull(conn, portBuf[:])
	if err != nil {
		return nil, fmt.Errorf("could not read dest port (in request header)")
	}
	destPort := binary.BigEndian.Uint16(portBuf[:])
	return &request{cmd: cmd, addressType: addressType, DestHost: destHost, DestPort: destPort}, nil
}

// +----+-----+-------+------+----------+----------+
// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
type reply struct {
	resCode     resultCode
	addressType addrType
	bindAddr    string
	bindPort    uint16
}

func (r *reply) marshal() ([]byte, error) {
	buf := []byte{
		socksServerVersion,
		byte(r.resCode),
		0,
		byte(r.addressType),
	}
	var bindAddrBinary []byte
	switch r.addressType {
	case ipv4:
		bindAddrBinary = net.ParseIP(r.bindAddr).To4()
		if bindAddrBinary == nil {
			return nil, fmt.Errorf("invalid IPv4 address (in reply header)")
		}
	case domainname:
		if len(r.bindAddr) > 255 {
			return nil, fmt.Errorf("invalid domain name (in reply header)")
		}
		bindAddrBinary = make([]byte, 0, len(r.bindAddr)+1)
		bindAddrBinary = append(bindAddrBinary, byte(len(r.bindAddr)))
		bindAddrBinary = append(bindAddrBinary, []byte(r.bindAddr)...)
	case ipv6:
		bindAddrBinary = net.ParseIP(r.bindAddr).To16()
		if bindAddrBinary == nil {
			return nil, fmt.Errorf("invalid IPv6 address (in replyl header)")
		}
	default:
		return nil, fmt.Errorf("invalid address type code -> (%v) <- (in reply header)", r.addressType)
	}
	var bindPortBinary [2]byte
	binary.BigEndian.PutUint16(bindPortBinary[:], r.bindPort)
	buf = append(buf, bindAddrBinary...)
	buf = append(buf, bindPortBinary[:]...)
	return buf, nil
}
