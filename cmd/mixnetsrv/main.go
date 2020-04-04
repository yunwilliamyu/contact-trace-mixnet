package main

import (
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io/ioutil"
	"log"
)

func main() {
	conf := &mixnet.MixnetConfig{}
	// TODO: load config
	key, err := ioutil.ReadFile("mykey")
	if err != nil {
		log.Fatal(err)
	}
	ms := mixnet.NewMixnetServer(conf, key)
	log.Fatal(ms.Run())
}
