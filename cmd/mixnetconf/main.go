package main

import (
	"encoding/json"
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/configs"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"log"
	"os"
)

var config = flag.String("config_file", "", "server config file, in json format")

func main() {
	flag.Parse()

	conf := &mixnet.MixnetServerConfig{}
	if err := configs.LoadConfig(*config, conf); err != nil {
		log.Fatal(err)
	}

	mc, err := mixnet.MakeClientConfig(conf)
	if err != nil {
		log.Fatal(err)
	}

	json.NewEncoder(os.Stdout).Encode(mc)
}
