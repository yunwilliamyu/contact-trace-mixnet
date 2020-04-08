package main

import (
	"encoding/json"
	"flag"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var config = flag.String("config", "", "JSON file with client configuration")

func configFromFlag() *mixnet.MixnetClientConfig {
	text, err := ioutil.ReadFile(*config)
	if err != nil {
		log.Fatal(err)
	}
	c := &mixnet.MixnetClientConfig{}
	if err := json.Unmarshal([]byte(text), c); err != nil {
		log.Fatal(err)
	}
	return c
}

func main() {
	flag.Parse()
	conf := configFromFlag()
	mc := mixnet.NewMixnetClient(conf)
	for {
		buf := make([]byte, mixnet.InnerMessageLength)
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
