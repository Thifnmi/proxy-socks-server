package main

import (
    "flag"
    "fmt"
    "net"
    "os"
    "strings"

    "github.com/joho/godotenv"
    "github.com/thifnmi/proxy-socks-server/logger"
    "github.com/thifnmi/proxy-socks-server/server"
    "github.com/thifnmi/proxy-socks-server/server/auth"
    "github.com/thifnmi/proxy-socks-server/utils"
)

func main() {
    // Load .env file
    _ = godotenv.Load()

    bindAddr := flag.String("addr", "0.0.0.0", "socks server bind address (optional)")
    bindPort := flag.String("port", "1081", "socks server bind port (optional)")
    dnsAddr := flag.String("dns", "", "specify a dns server (ip:port) to be used for resolving domains (optional)")
    flag.Parse()

    var resolver utils.Resolver
    if *dnsAddr == "" {
        resolver = utils.DefaultResolver{}
    } else {
        if _, _, err := net.SplitHostPort(*dnsAddr); err != nil {
            logger.Info("DNS server should be in this format 'ip:port'")
            return
        }
        logger.Infof("DNS server %v", *dnsAddr)
        resolver = utils.NewCustomResolver(*dnsAddr)
    }

    // Get credentials from environment variables
    userList := os.Getenv("SOCKS_USERS")
    passList := os.Getenv("SOCKS_PASSWORDS")
    if userList == "" || passList == "" {
        logger.Info("SOCKS_USERS and SOCKS_PASSWORDS must be set in .env file")
        return
    }
    usernames := strings.Split(userList, ",")
    passwords := strings.Split(passList, ",")

    if len(usernames) != len(passwords) {
        logger.Info("SOCKS_USERS and SOCKS_PASSWORDS must have the same number of entries")
        return
    }

    creds := auth.StaticCredentials{}
    for i := range usernames {
        creds[usernames[i]] = passwords[i]
    }
    cator := auth.UserPassAuthenticator{Credentials: creds}

    config := &utils.Config{
        AuthMethods: []auth.Authenticator{cator},
        Credentials: creds,
        Resolv:      resolver,
    }
    bindListenner := fmt.Sprintf("%s:%s", *bindAddr, *bindPort)

    s := server.NewSocksServer(config)
    err := s.ListenAndServe("tcp", bindListenner)
    if err != nil {
        logger.Infof("Failed to listen socks server: %s", err)
    }
}