package mixnet

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/yunwilliamyu/contact-trace-mixnet/rand"
	"golang.org/x/crypto/hkdf"
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

type MixnetConfig struct {
	MinBatch int
	// reverse indexed!
	Addrs   []string
	PubKeys [][32]byte // [0] is unused
}

func (mc MixnetConfig) URL(idx int) string {
	return fmt.Sprintf("http://%s/receive", mc.Addrs[idx])
}

type keys struct {
	privateKey [32]byte
	publicKey  [32]byte
}

func forwardMessageLength(idx int) int {
	return InnerMessageLength + box.AnonymousOverhead*(idx+1)
}

func (k keys) forwardTransformOnion(msg []byte) ([]byte, error) {
	decMsg, ok := box.OpenAnonymous(nil, msg, &k.publicKey, &k.privateKey)
	if !ok {
		return nil, fmt.Errorf("received invalid message") // invalid message, ignore
	}
	return decMsg, nil
}

// MixnetServer represents a nonfinal server in the mixnet chain
type MixnetServer struct {
	conf           *MixnetConfig
	idx            int // how many servers are there in front of me, incl. the final endpoint
	keys           keys
	MessageHandler func([]byte)
	// next server address/connection to it

	onions [][]byte // messages to forward, already decrypted
	mu     sync.Mutex
}

const InnerMessageLength = 10 // TODO

// TODO: sane logging prefixes
func (ms *MixnetServer) name() string {
	return ms.conf.Addrs[ms.idx]
}

func (ms *MixnetServer) Receive(msg []byte) (ok bool) {
	// TODO: do we need to ensure that only the previous server is talking to us? probably, because
	if len(msg) != forwardMessageLength(ms.idx) {
		log.Printf("received message of invalid length")
		return false
	}
	decMsg, err := ms.keys.forwardTransformOnion(msg)
	if err != nil {
		log.Printf("received invalid message: %s", err.Error())
		return false
	}
	if len(decMsg) != forwardMessageLength(ms.idx-1) {
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

	msgSize := forwardMessageLength(ms.idx)
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
	allOnions := make([]byte, 0, len(onions)*forwardMessageLength(ms.idx-1))
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
		if count > ms.conf.MinBatch {
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

func deriveKeys(masterKey string) keys {
	var k keys
	onionDeriver := hkdf.New(sha256.New, []byte(masterKey), nil, []byte("ONION_KEY"))
	var err error
	onionPubKey, onionPrivKey, err := box.GenerateKey(onionDeriver)
	if err != nil {
		log.Fatal(err)
	}
	k.publicKey = *onionPubKey
	k.privateKey = *onionPrivKey
	return k
}

func NewMixnetServer(conf *MixnetConfig, masterKey string) *MixnetServer {
	ms := &MixnetServer{conf: conf}
	ms.keys = deriveKeys(masterKey)

	ms.idx = -1
	for i, pk := range conf.PubKeys {
		if pk == ms.keys.publicKey {
			ms.idx = i
			break
		}
	}
	if ms.idx == -1 {
		panic("cannot find our key in list")
	}
	return ms
}

func PubKey(masterKey string) [32]byte {
	keys := deriveKeys(masterKey)
	return keys.publicKey
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
