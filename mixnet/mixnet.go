package mixnet

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/yunwilliamyu/contact-trace-mixnet/mixnet/pb"
	"github.com/yunwilliamyu/contact-trace-mixnet/rand"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"io/ioutil"
	"log"
	mathrand "math/rand"
	"net/http"
	"sync"
	"time"
)

// TODO: TLS
// TODO: maybe non-http if we need something in any way more complicated

type MixnetClientConfig struct {
	Addr          string
	PubKeys       [][32]byte // reverse indexed!
	MessageLength int
}

type MixnetServerConfig struct {
	Addrs               []string
	OutputAddr          string
	OtpCheck            string
	MinBatchSize        int `json:"min_batch_size"`
	MessageLength       int `json:"message_length"`
	MaxBufferedMessages int `json:"max_buffered_messages"`
}

func (msc MixnetServerConfig) NextAddr(idx int) string {
	return msc.Addrs[idx-1]
}

func (msc MixnetServerConfig) InputMessageLength(idx int) int {
	return ForwardMessageLength(idx, msc.MessageLength)
}

func sendURL(addr string) string {
	return fmt.Sprintf("%s/v0/receive", addr)
}

type keys struct {
	privateKey [32]byte
	publicKey  [32]byte
}

func ForwardMessageLength(idx int, messageLength int) int {
	return messageLength + box.AnonymousOverhead*(idx+1)
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
	conf        *MixnetServerConfig
	idx         int
	keys        keys
	otpChecker  *OTPChecker
	PushHandler func([][]byte) error
	// next server address/connection to it

	onions      [][]byte // messages to forward, already decrypted
	mu          sync.Mutex
	readyToPush *sync.Cond
}

func (ms *MixnetServer) checkOTP(req *pb.PutOnionsRequest) error {
	if ms.otpChecker == nil {
		return nil
	}
	if req.GetOtp() == "" {
		// provide a clearer error
		return fmt.Errorf("no OTP provided")
	}
	return ms.otpChecker.Check(req.GetOtp(), req.GetCxid())
}

func (ms *MixnetServer) Receive(req *pb.PutOnionsRequest) error {
	// TODO: do we need to ensure that only the previous server is talking to us? probably, because
	ms.mu.Lock()
	// do not bother decrypting if we want to refuse anyway
	messageCount := len(ms.onions) + len(req.Msgs)
	ms.mu.Unlock()

	if messageCount > ms.conf.MaxBufferedMessages {
		return fmt.Errorf("too many buffered messages")
	}

	if err := ms.checkOTP(req); err != nil {
		return err
	}

	// TODO: actually enforce the message count limit, not only best-effortish
	for _, msg := range req.Msgs {
		if len(msg) != ms.conf.InputMessageLength(ms.idx) {
			log.Printf("received message of invalid length")
			continue
		}
		decMsg, err := ms.keys.forwardTransformOnion(msg)
		if err != nil {
			log.Printf("received invalid message: %s", err.Error())
			continue
		}
		ms.addMessage(decMsg)
	}
	return nil
}

func (ms *MixnetServer) ServePubkey(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.WriteHeader(http.StatusOK)
	rw.Write(ms.keys.publicKey[:])
}

func (ms *MixnetServer) ServeConfig(rw http.ResponseWriter, req *http.Request) {
	text, err := json.MarshalIndent(ms.conf, "", "  ")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write(text)
}

func (ms *MixnetServer) ServeReceive(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "only POST allowed", http.StatusBadRequest)
		return
	}
	ct := req.Header.Get("Content-Type")
	switch ct {
	case "application/octet-stream":
		ms.legacyReceive(rw, req)
	case "application/json":
		ms.jsonReceive(rw, req)
	default:
		http.Error(rw, fmt.Sprintf("unknown request Content-Type %q", ct), http.StatusBadRequest)
	}
}

//go:generate protoc pb/mixnet.proto --go_out=. --go_opt=paths=source_relative

func (ms *MixnetServer) jsonReceive(rw http.ResponseWriter, req *http.Request) {
	contents, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		http.Error(rw, fmt.Sprintf("couldn't read request: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	putReq := &pb.PutOnionsRequest{}
	if err := protojson.Unmarshal(contents, putReq); err != nil {
		http.Error(rw, fmt.Sprintf("couldn't parse request: %s", err.Error()), http.StatusBadRequest)
		return
	}
	if err := ms.Receive(putReq); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusAccepted)
}

func (ms *MixnetServer) legacyReceive(rw http.ResponseWriter, req *http.Request) {
	putReq := &pb.PutOnionsRequest{}
	for {
		msg := make([]byte, ms.conf.InputMessageLength(ms.idx))
		if _, err := io.ReadFull(req.Body, msg); err != nil {
			if err == io.EOF {
				break
			}
			http.Error(rw, fmt.Sprintf("cannot read full message: %s", err.Error()), http.StatusBadRequest)
			return
		}
		putReq.Msgs = append(putReq.Msgs, msg)
	}
	if err := ms.Receive(putReq); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusAccepted)
}

func (ms *MixnetServer) push(onions [][]byte) error {
	// TODO: this reads urandom. make this read csprng
	rng := mathrand.New(rand.ReaderSource{cryptorand.Reader})
	rng.Shuffle(len(onions), func(i, j int) {
		onions[i], onions[j] = onions[j], onions[i]
	})

	req := &pb.PutOnionsRequest{
		Msgs: onions,
	}

	rawReq, err := protojson.Marshal(req)
	if err != nil {
		return err
	}

	// send to next
	// TODO: cache clients/connections/something
	var url string
	if ms.idx > 0 {
		url = sendURL(ms.conf.NextAddr(ms.idx))
	} else {
		url = sendURL(ms.conf.OutputAddr)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(rawReq))
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d (%s) from /receive", resp.StatusCode, resp.Status)
	}
	return nil
}

func (ms *MixnetServer) loop() {
	for {
		var toSend [][]byte
		ms.mu.Lock()
		for len(ms.onions) < ms.conf.MinBatchSize {
			ms.readyToPush.Wait()
		}
		// we want some limit, but probably a larger one
		toSend = ms.onions[:ms.conf.MinBatchSize]
		ms.mu.Unlock()

		toSend = append([][]byte(nil), toSend...) // we want to be able to write to toSend

		log.Printf("pushing %d onions", len(toSend))
		var err error
		if ms.PushHandler != nil {
			err = ms.PushHandler(toSend)
		} else {
			err = ms.push(toSend)
		}
		if err == nil {
			log.Printf("push successful")
			ms.mu.Lock()
			ms.onions = ms.onions[len(toSend):]
			ms.mu.Unlock()
		} else {
			log.Printf("error while pushing: %s", err.Error())
			// TODO: reasonable backoffs for retrying
			time.Sleep(10 * time.Second)
		}
		// TODO: terminate the loop sometime
	}
}

func (ms *MixnetServer) addMessage(msg []byte) {
	ms.mu.Lock()
	ms.onions = append(ms.onions, msg)
	if len(ms.onions) >= ms.conf.MinBatchSize {
		ms.readyToPush.Signal()
	}
	ms.mu.Unlock()
}

func (ms *MixnetServer) Run(listenAddr string) error {
	go ms.loop()

	mux := http.NewServeMux()
	mux.Handle("/v0/receive", http.HandlerFunc(ms.ServeReceive))
	mux.Handle("/v0/pubkey", http.HandlerFunc(ms.ServePubkey))
	mux.Handle("/v0/config", http.HandlerFunc(ms.ServeConfig))

	s := &http.Server{
		Addr:    listenAddr,
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

func NewMixnetServer(conf *MixnetServerConfig, idx int, masterKey string) *MixnetServer {
	ms := &MixnetServer{conf: conf, idx: idx}
	ms.keys = deriveKeys(masterKey)
	ms.readyToPush = sync.NewCond(&ms.mu)
	if conf.OtpCheck != "" && idx == len(conf.Addrs)-1 {
		ms.otpChecker = NewOTPChecker(conf.OtpCheck)
	}
	return ms
}

func PubKey(masterKey string) [32]byte {
	keys := deriveKeys(masterKey)
	return keys.publicKey
}

type MixnetClient struct {
	conf *MixnetClientConfig
}

func NewMixnetClient(conf *MixnetClientConfig) *MixnetClient {
	return &MixnetClient{conf: conf}
}

func (mc *MixnetClient) SendMessage(msg []byte) error {
	if len(msg) != mc.conf.MessageLength {
		return fmt.Errorf("wrong message size: %d!=%d", len(msg), mc.conf.MessageLength)
	}
	onion := msg
	for _, pk := range mc.conf.PubKeys {
		var err error
		// TODO: decrease allocations: every second Seal can use the same output buffer
		onion, err = box.SealAnonymous(nil, onion, &pk, cryptorand.Reader)
		if err != nil {
			return err
		}
	}
	resp, err := http.Post(sendURL(mc.conf.Addr), "application/octet-stream", bytes.NewReader(onion))
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode > 400 {
		return fmt.Errorf("status %d (%s) from /receive", resp.StatusCode, resp.Status)
	}
	return nil
}

func MakeClientConfig(sc *MixnetServerConfig) (*MixnetClientConfig, error) {
	conf := &MixnetClientConfig{
		Addr:          sc.Addrs[len(sc.Addrs)-1],
		PubKeys:       make([][32]byte, len(sc.Addrs)),
		MessageLength: sc.MessageLength,
	}
	// TODO: do in parallel
	for i, addr := range sc.Addrs {
		resp, err := http.Get(fmt.Sprintf("%s/v0/pubkey", addr))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("received %d (%s) from %s", resp.StatusCode, resp.Status, addr)
		}
		pubkey, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(pubkey) != 32 {
			return nil, fmt.Errorf("key received from %s is %d bytes long instead of %d", addr, len(pubkey), 32)
		}
		copy(conf.PubKeys[i][:], pubkey)
	}
	return conf, nil
}
