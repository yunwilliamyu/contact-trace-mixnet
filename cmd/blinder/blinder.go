package main

import (
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/blinding"
	"log"
)

var listenAddr = flag.String("listen_addr", ":8787", "address to listen on")
var keyDir = flag.String("key_dir", "", "directory to read keys from")

func main() {
	flag.Parse()

	kr := blinding.KeyReader{Dir: *keyDir}
	b := blinding.New(kr.ReadKey)
	log.Fatal(b.Run(*listenAddr))
}
