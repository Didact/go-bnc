package main

import (
	"flag"
	"fmt"
)

// flags
var (
	prefix    = flag.String("prefix", "bnc-", "prefix for commands sent to the bnc instead of the server")
	server    = flag.String("server", "", "server address to connect to")
	localPort = flag.Int("localPort", 3434, "local port to listen on")
	verbose   = flag.Bool("verbose", false, "verbose logging on/off")
)

func main() {
	fmt.Println("Hello, world!")
}
