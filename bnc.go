package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

var RN = []byte{'\r', '\n'}

// flags
var (
	prefix    = flag.String("prefix", "bnc-", "prefix for commands sent to the bnc instead of the server")
	server    = flag.String("server", "", "server address to connect to")
	localPort = flag.Int("localPort", 3434, "local port to listen on")
	verbose   = flag.Bool("verbose", false, "verbose logging on/off")
)

// clientConn represents a connection from a client to the bnc
type clientConn struct {
	net.Conn
	buf []byte
}

func (c *clientConn) Read(p []byte) (n int, err error) {
	if len(c.buf) == 0 {
		c.buf = make([]byte, len(p))
		n, err := c.Conn.Read(c.buf)
		if err != nil {
			return n, err
		}
		c.buf = c.buf[:n]
	}
	i := bytes.Index(c.buf, RN)
	if i < 0 {
		return 0, errors.New("wtf")
	}
	//log.Printf("input: %s", c.buf[:i+len(RN)])
	message := c.buf[:i+len(RN)]
	c.buf = c.buf[i+len(RN):]

	if bytes.HasPrefix(message, []byte("PING")) {
		message[1] = 'O'
		c.Conn.Write(message)
		return c.Read(p)
	}

	b := c.process(message)
	if len(b) == 0 {
		return c.Read(p)
	}
	n = copy(p, b)
	return n, nil
}

func (c *clientConn) process(p []byte) []byte {
	if bytes.HasPrefix(p, []byte("QUIT")) {
		log.Println("QUIT")
		return nil
	}

	if bytes.HasPrefix(p, []byte("NICK")) {
		if nick != nil {
			log.Println("non-nil nick")
			return nil
		}
		nick = new(string)
		*nick = string(p[5:])
	}
	return p
}

type serverConn struct {
	net.Conn
	buf []byte
}

func (s *serverConn) Read(p []byte) (n int, err error) {
	n, err = s.Conn.Read(p)
	if bytes.HasPrefix(p, []byte("PING")) {
		p[1] = 'O'
		s.Conn.Write(p[:n])

		// initially I just returned 0, nil, but go's stdlib (correctly) realizes
		// that multiple Read calls returning 0, nil means something is fishy
		// small chance of stack overflow if you get nothing but PING's, I guess
		return s.Read(p)
	}
	return n, err
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
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
		}
		once.Do(func() {
			wait <- struct{}{}
		})
		clientsChan <- conn
	}()

	<-wait

	server, err := net.Dial("tcp", *server)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for client := range clientsChan {
			clients = append(clients, client)
			wrapper := &clientConn{client, nil}
			tee := io.TeeReader(wrapper, os.Stdout)
			go io.Copy(server, tee)
		}
	}()

	// 512 is the maximum message size according to RFC 1459
	r := bufio.NewReaderSize(&serverConn{server, nil}, 512)
	for {
		b, _, err := r.ReadLine()
		if err != nil {
			log.Fatal(err)
		}
		for _, conn := range clients {
			fmt.Println(string(b))
			conn.Write(b)
			conn.Write(RN)
		}
	}

	select {}
}
