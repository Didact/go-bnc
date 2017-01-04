package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// flags
var (
	prefix    = flag.String("prefix", "bnc-", "prefix for commands sent to the bnc instead of the server")
	server    = flag.String("server", "", "server address to connect to")
	localPort = flag.Int("localPort", 3434, "local port to listen on")
	verbose   = flag.Bool("verbose", false, "verbose logging on/off")
)

// conn represents a connection from a client to the bnc
type conn struct {
	net.Conn
}

func (c *conn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	fmt.Println(string(p[:n]))

	if err != nil {
		fmt.Println("non-nil err")
		return n, err
	}

	// assume each read contains one message
	// tbh I'm not sure would I should do with p in this case
	if bytes.HasPrefix(p, []byte("QUIT")) {
		fmt.Println("QUIT")
		return 0, nil
	}

	if bytes.HasPrefix(p, []byte("NICK")) {
		if nick != nil {
			fmt.Println("non-nil nick")
			return 0, nil
		}
		nick = new(string)
		*nick = string(p[5:])
	}
	return n, nil
}

// if nick is nil we should honor the first NICK sent
// otherwise they'll require the prefix
var nick *string

var clients []net.Conn
var clientsChan = make(chan net.Conn)

func main() {
	flag.Parse()
	if *server == "" {
		log.Fatal("invalid server address")
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *localPort))
	if err != nil {
		log.Fatal(err)
	}

	wait := make(chan struct{})
	once := &sync.Once{}

	go func() {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Println(err)
		}
		once.Do(func() {
			wait <- struct{}{}
		})
		clientsChan <- clientConn
	}()

	<-wait

	server, err := net.Dial("tcp", *server)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for client := range clientsChan {
			clients = append(clients, client)
			wrapper := &conn{client}
			go io.Copy(server, wrapper)
		}
	}()

	// 512 is the maximum message size according to RFC 1459
	r := bufio.NewReaderSize(server, 512)
	for {
		b, _, err := r.ReadLine()
		if err != nil {
			log.Fatal(err)
		}
		for _, conn := range clients {
			conn.Write(b)
			conn.Write([]byte("\r\n"))
		}
	}

	select {}
}
