package main

import (
        "bytes"
        "encoding/binary"
        "flag"
        "fmt"
        "io"
        "net"
        "strconv"
        "strings"
        "time"
)

var socks5Addr string

func parseTarget(targetAddr string) (net.IP, uint16, error) {
        targetArr := strings.Split(targetAddr, ":")
        if len(targetArr) != 2 {
                return nil, 0, fmt.Errorf("invalid target address")
        }
        tmpip := net.ParseIP(targetArr[0])
        if tmpip == nil {
                return nil, 0, fmt.Errorf("invalid target ip")
        }
        targetIP := tmpip.To4()
        if targetIP == nil {
                return nil, 0, fmt.Errorf("invalid target ip, just support ipv4")
        }
        targetPort, err := strconv.ParseUint(targetArr[1], 10, 16)
        if err != nil {
                return nil, 0, err
        }
        return targetIP, uint16(targetPort), nil
}

func copy(dst net.Conn, src net.Conn) {
        var tmp [1024]byte
        for {
                n, err := src.Read(tmp[:])
                if err != nil {
                        src.Close()
                        dst.Close()
                        return
                }
                writedbytes := 0
                for writedbytes < n {
                        tmpn, err := dst.Write(tmp[writedbytes:n])
                        if err != nil {
                                src.Close()
                                dst.Close()
                                return
                        }
                        writedbytes += tmpn
                }
        }
}

func main() {
        var listenAddr string
        var targetAddr string
        flag.StringVar(&socks5Addr, "socks5", "", "socks5 proxy address")
        flag.StringVar(&listenAddr, "listen", "", "listen address")
        flag.StringVar(&targetAddr, "target", "", "target address")
        flag.Parse()
        if len(socks5Addr) == 0 || len(listenAddr) == 0 || len(targetAddr) == 0 {
                flag.Usage()
                return
        }
        targetIP, targetPort, err := parseTarget(targetAddr)
        if err != nil {
                fmt.Println(err)
                return
        }
        listener, err := net.Listen("tcp4", listenAddr)
        if err != nil {
                fmt.Println(err)
                return
        }
        for {
                conn, err := listener.Accept()
                if err != nil {
                        fmt.Println(err)
                        continue
                }
                go newConn(conn, targetIP, targetPort)
        }
}

func newConn(conn net.Conn, targetIP net.IP, targetPort uint16) {
        proxyConn, err := net.DialTimeout("tcp4", socks5Addr, 6*time.Second)
        if err != nil {
                fmt.Println(err)
                conn.Close()
                return
        }
        proxyConn.Write([]byte{5, 1, 0})
        var rsp [2]byte
        if _, err := io.ReadFull(proxyConn, rsp[:]); err != nil {
                fmt.Println(err)
                conn.Close()
                proxyConn.Close()
                return
        }
        if rsp[0] != 5 || rsp[1] != 0 {
                conn.Close()
                proxyConn.Close()
                return
        }
        var connect bytes.Buffer
        connect.Write([]byte{5, 1, 0, 1})
        connect.Write(targetIP)
        var portBytes [2]byte
        binary.BigEndian.PutUint16(portBytes[:], targetPort)
        connect.Write(portBytes[:])
        proxyConn.Write(connect.Bytes())
        var connectRsp [10]byte
        if _, err := io.ReadFull(proxyConn, connectRsp[:]); err != nil {
                proxyConn.Close()
                conn.Close()
                return
        }
        if connectRsp[0] != 5 || connectRsp[1] != 0 {
                proxyConn.Close()
                conn.Close()
                return
        }
        go func() {
                copy(conn, proxyConn)
        }()
        copy(proxyConn, conn)
}
