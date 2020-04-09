package mixnet

import (
	"fmt"
	"log"
	"testing"
	"time"
)

const messageLength = 10

func msgForId(i int) [messageLength]byte {
	var msg [messageLength]byte
	msg[0] = byte(i)
	return msg
}

func TestSmoke(t *testing.T) {
	const depth = 3
	masterKeys := make([]string, depth)
	for i := range masterKeys {
		masterKeys[i] = fmt.Sprintf("key%d", i)
	}
	msc := &MixnetServerConfig{
		MinBatchSize:        10,
		MessageLength:       messageLength,
		MaxBufferedMessages: 1000,
		Addrs:               make([]string, depth),
	}
	addrs := make([]string, depth)
	for i := range masterKeys {
		addrs[i] = fmt.Sprintf("127.0.0.1:%d", 8000+i)
		msc.Addrs[i] = "http://" + addrs[i]
	}
	recv := make(chan string, 1)
	for i := range masterKeys {
		go func(i int) {
			ms := NewMixnetServer(msc, i, masterKeys[i])
			if i == 0 {
				ms.MessageHandler = func(msg []byte) {
					fmt.Printf("msg: %v\n", msg)
					recv <- string(msg)
				}
			}
			err := ms.Run(addrs[i])
			log.Fatal(err)
		}(i)
	}

	// TODO: the following races with the servers starting to listen
	// we should either synchronize this test, or do healthchecking and waiting for healthiness
	// Or, we just replace this with an actual rpc framework and delegate that.
	mc, err := MakeClientConfig(msc)
	if err != nil {
		t.Fatal(err)
	}

	cl := NewMixnetClient(mc)

	const count = 10
	sent := make(map[string]bool)
	for i := 0; i < count; i++ {
		msg := msgForId(i)
		sent[string(msg[:])] = true
		err := cl.SendMessage(msg[:])
		if err != nil {
			t.Errorf("SendMessage: %s", err.Error())
		}
	}

	stop := make(chan struct{})

	go func() {
		var dummyMsg [messageLength]byte
		for {
			time.Sleep(10 * time.Millisecond)
			_ = cl.SendMessage(dummyMsg[:])
			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	// TODO: do not block the last mixer while sending

	for msg := range recv {
		if len(sent) == 0 {
			break
		}
		if !sent[msg] {
			continue
		}
		delete(sent, msg)
	}
	close(stop)

}
