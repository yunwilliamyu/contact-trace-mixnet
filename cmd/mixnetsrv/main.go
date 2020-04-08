package main

import (
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/configs"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

var masterKeyFile = flag.String("master_key_file", "PROVIDE MASTER KEY", "Path to the master secret key")
var listenAddr = flag.String("listen_addr", "PROVIDE LISTEN ADDR", "Address to bind to")
var idx = flag.Int("idx", 0, "Index in the mixnet, counting from the end") // TODO: relieve the need to specify this
var config = flag.String("config_file", "", "path to the location of the config file in json format")

func main() {
	flag.Parse()

	conf := &mixnet.MixnetServerConfig{}
	if err := configs.LoadConfig(*config, conf); err != nil {
		log.Fatal(err)
	}

	masterKey, err := ioutil.ReadFile(*masterKeyFile)
	if err != nil {
		log.Fatal(err)
	}

	ms := mixnet.NewMixnetServer(conf, *idx, string(masterKey))
	if *idx == 0 {
		var mu sync.Mutex
		ms.MessageHandler = func(msg []byte) {
			mu.Lock()
			os.Stdout.Write(msg) // TODO: nondelimited output here seems like a very bad idea
			mu.Unlock()
		}
	}
	log.Fatal(ms.Run(*listenAddr))
}
