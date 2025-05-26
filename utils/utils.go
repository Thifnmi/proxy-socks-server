package utils

import (
	"context"
	"github.com/thifnmi/proxy-socks-server/server/auth"
	"net"
	"time"
)

type Config struct {
	AuthMethods []auth.Authenticator
	Credentials auth.CredentialStore
	Resolv      Resolver
	Dial        func(ctx context.Context, network, addr string) (net.Conn, error)
}

type Resolver interface {
	Resolve(ctx context.Context, name string) (net.IP, error)
}

type DefaultResolver struct{}

func (r DefaultResolver) Resolve(ctx context.Context, name string) (net.IP, error) {
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return nil, err
	}
	return addr.IP, nil
}

type CustomResolver struct {
	netResolver *net.Resolver
}

func NewCustomResolver(dnsAddr string) *CustomResolver {
	return &CustomResolver{
		netResolver: &net.Resolver{
			PreferGo: true,
			Dial: func(_ context.Context, network, address string) (net.Conn, error) {
				return net.DialTimeout(network, dnsAddr, 3*time.Second)
			},
		},
	}
}

func (d *CustomResolver) Resolve(ctx context.Context, name string) (net.IP, error) {
	ips, err := d.netResolver.LookupIP(ctx, "ip", name)
	if err != nil {
		return nil, err
	}
	return ips[0], nil
}
