package mixnet

import (
	"bytes"
	cryptorand "crypto/rand"
	"fmt"
	"github.com/yunwilliamyu/contact-trace-mixnet/rand"
	"golang.org/x/crypto/nacl/box"
	"io"
	"log"
	mathrand "math/rand"
	"net/http"
	"sync"
	"time"
)

// TODO: TLS
// TODO: maybe non-http if we need something in any way more complicated

const MinBatch = 4 // TODO: realistic value

type MixnetConfig struct {
	// reverse indexed!
	Addrs   []string
	PubKeys [][32]byte // [0] is unused
}

func (mc MixnetConfig) URL(idx int) string {
	return fmt.Sprintf("http://%s/receive", mc.Addrs[idx])
}

// MixnetServer represents a nonfinal server in the mixnet chain
type MixnetServer struct {
	conf           *MixnetConfig
	idx            int // how many servers are there in front of me, incl. the final endpoint
	privateKey     [32]byte
	publicKey      [32]byte
	MessageHandler func([]byte)
	// next server address/connection to it

	onions [][]byte // messages to forward, already decrypted
	mu     sync.Mutex
}

const InnerMessageLength = 10 // TODO

func messageLength(idx int) int {
	return InnerMessageLength + box.AnonymousOverhead*(idx+1)
}

// TODO: sane logging prefixes
func (ms *MixnetServer) name() string {
	return ms.conf.Addrs[ms.idx]
}

func (ms *MixnetServer) Receive(msg []byte) (ok bool) {
	if len(msg) != messageLength(ms.idx) {
		log.Printf("received message of invalid length")
		return false
	}
	// TODO: do we need to ensure that only the previous server is talking to us? probably, because
	decMsg, ok := box.OpenAnonymous(nil, msg, &ms.publicKey, &ms.privateKey)
	if !ok {
		log.Printf("%s: received invalid message", ms.name())
		return false // invalid message, ignore
	}
	if len(decMsg) != messageLength(ms.idx-1) {
		panic(len(decMsg))
	}
	ms.addMessage(decMsg)
	return true
}

func (ms *MixnetServer) addMessage(msg []byte) {
	if ms.idx == 0 {
		ms.MessageHandler(msg)
		return
	}
	ms.mu.Lock()
	ms.onions = append(ms.onions, msg)
	ms.mu.Unlock()
}

func (ms *MixnetServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "only POST allowed", http.StatusBadRequest)
		return
	}

	msgSize := messageLength(ms.idx)
	for {
		msg := make([]byte, msgSize)
		if _, err := io.ReadFull(req.Body, msg); err != nil {
			if err == io.EOF {
				break
			}
			http.Error(rw, fmt.Sprintf("cannot read full message: %s", err.Error()), http.StatusBadRequest)
			return
		}
		if !ms.Receive(msg) {
			log.Printf("cannot decrypt an incoming message; ignoring")
		}
	}
	rw.WriteHeader(http.StatusAccepted)
}

func (ms *MixnetServer) push() {
	ms.mu.Lock()
	onions := ms.onions
	ms.onions = nil
	ms.mu.Unlock()
	log.Printf("pushing (len=%d)", len(onions))

	// TODO: this reads urandom. make this read csprng
	rng := mathrand.New(rand.ReaderSource{cryptorand.Reader})
	rng.Shuffle(len(onions), func(i, j int) {
		onions[i], onions[j] = onions[j], onions[i]
	})

	// concatenate all the onions
	allOnions := make([]byte, 0, len(onions)*messageLength(ms.idx-1))
	for _, o := range onions {
		allOnions = append(allOnions, o...)
	}
	// send to next
	// TODO: cache clients/connections/something
	if _, err := http.Post(ms.conf.URL(ms.idx-1), "application/octet-stream", bytes.NewReader(allOnions)); err != nil {
		// TODO: http error codes do not provide error iirc
		// TODO: retry?
		log.Printf("error sending further: %s", err)
		return
	}
	log.Printf("pushed")
}

func (ms *MixnetServer) loop() {
	for {
		ms.mu.Lock()
		count := len(ms.onions)
		ms.mu.Unlock()
		if count > MinBatch {
			ms.push()
		}
		time.Sleep(time.Millisecond)
		// TODO: terminate the loop sometime
	}
}

func (ms *MixnetServer) Run() error {
	go ms.loop()

	mux := http.NewServeMux()
	mux.Handle("/receive", ms)

	s := &http.Server{
		Addr:    ms.conf.Addrs[ms.idx],
		Handler: mux,
	}
	return s.ListenAndServe()
}

func NewMixnetServer(conf *MixnetConfig, key []byte) *MixnetServer {
	ms := &MixnetServer{conf: conf}
	copy(ms.privateKey[:], key[0:32])
	copy(ms.publicKey[:], key[32:])

	ms.idx = -1
	for i, pk := range conf.PubKeys {
		if pk == ms.publicKey {
			ms.idx = i
			break
		}
	}
	if ms.idx == -1 {
		panic("cannot find our key in list")
	}
	return ms
}

func GenerateKeypair() []byte {
	pubKey, privKey, err := box.GenerateKey(cryptorand.Reader)
	if err != nil {
		panic(err)
	}
	r := make([]byte, 32*2)
	copy(r[0:32], privKey[:])
	copy(r[32:], pubKey[:])
	return r
}

func PubKey(keypair []byte) [32]byte {
	var r [32]byte
	copy(r[:], keypair[32:])
	return r
}

type MixnetClient struct {
	conf *MixnetConfig
}

func NewMixnetClient(conf *MixnetConfig) *MixnetClient {
	return &MixnetClient{conf: conf}
}

func (mc *MixnetClient) SendMessage(msg []byte) error {
	if len(msg) != InnerMessageLength {
		return fmt.Errorf("wrong message size: %d!=%d", len(msg), InnerMessageLength)
	}
	onion := msg
	for _, pk := range mc.conf.PubKeys {
		var err error
		onion, err = box.SealAnonymous(nil, onion, &pk, cryptorand.Reader)
		if err != nil {
			return err
		}
	}
	http.Post(mc.conf.URL(len(mc.conf.Addrs)-1), "application/octet-stream", bytes.NewReader(onion))
	return nil
}
