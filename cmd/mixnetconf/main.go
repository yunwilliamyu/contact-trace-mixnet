package main

import (
	"encoding/json"
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io/ioutil"
	"log"
	"os"
)

var config = flag.String("config_file", "", "server config file, in json format")

func configFromFlag() *mixnet.MixnetServerConfig {
	text, err := ioutil.ReadFile(*config)
	if err != nil {
		log.Fatal(err)
	}
	c := &mixnet.MixnetServerConfig{}
	if err := json.Unmarshal(text, &c); err != nil {
		log.Fatal(err)
	}
	return c
}

func main() {
	flag.Parse()

	conf := configFromFlag()

	mc, err := mixnet.MakeClientConfig(conf.Addrs)
	if err != nil {
		log.Fatal(err)
	}

	mc.MessageLength = conf.MessageLength

	json.NewEncoder(os.Stdout).Encode(mc)
}
