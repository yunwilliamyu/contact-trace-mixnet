package main

import (
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io/ioutil"
	"log"
)

func main() {
	conf := &mixnet.MixnetConfig{}
	// TODO: load config
	masterKey, err := ioutil.ReadFile("mykey")
	if err != nil {
		log.Fatal(err)
	}
	ms := mixnet.NewMixnetServer(conf, string(masterKey))
	log.Fatal(ms.Run())
}
