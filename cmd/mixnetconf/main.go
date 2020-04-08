package main

import (
	"flag"
	"encoding/json"
	"os"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"log"
)

func main() {
	flag.Parse()
	addrs := flag.Args()

	mc, err := mixnet.MakeClientConfig(addrs)
	if err != nil {
		log.Fatal(err)
	}

	json.NewEncoder(os.Stdout).Encode(mc)
}
