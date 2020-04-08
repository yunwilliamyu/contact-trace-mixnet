package main

import (
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/configs"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io"
	"log"
	"os"
)

var config = flag.String("config", "", "JSON file with client configuration")

func main() {
	flag.Parse()

	conf := &mixnet.MixnetClientConfig{}
	if err := configs.LoadConfig(*config, conf); err != nil {
		log.Fatal(err)
	}

	mc := mixnet.NewMixnetClient(conf)
	for {
		buf := make([]byte, conf.MessageLength)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		if err := mc.SendMessage(buf); err != nil {
			log.Print(err)
		}
	}
}
