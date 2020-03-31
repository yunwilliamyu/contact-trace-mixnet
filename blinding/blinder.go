package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/hkdf"
	"io"
	"log"
	"net/http"
	"sort"
	"unsafe"
)

// #cgo LDFLAGS: -lsodium
// #include <sodium.h>
import "C"

type BlindingKey struct {
	blindingKey [C.crypto_core_ristretto255_SCALARBYTES]byte
}

func NewBlindingKey(masterKey string) *BlindingKey {
	bk := &BlindingKey{}
	blindingDeriver := hkdf.New(sha256.New, []byte(masterKey), nil, []byte("BLINDING_KEY"))
	_, err := io.ReadFull(blindingDeriver, bk.blindingKey[:])
	if err != nil {
		panic(err)
	}
	// we can derive additional keys, if we need them, in the same way
	return bk
}

func (bk *BlindingKey) permute(values [][]byte) {
	// This can be deterministic if need be: we can use some key derived
	// from the same master key blindingKey is derived from, together with
	// a hash of the request, to seed a CPRNG and use that to permute.
	for i := 0; i < len(values); i++ {
		j := int(C.randombytes_random()) % (i + 1) // there is bias, but it's small
		values[i], values[j] = values[j], values[i]
	}
}

func (bk *BlindingKey) exponentiate(input []byte) ([]byte, error) {
	if len(input) != C.crypto_core_ristretto255_BYTES {
		return nil, errors.New("invalid length of curve point")
	}
	output := make([]byte, C.crypto_core_ristretto255_BYTES)
	ret := C.crypto_scalarmult_ristretto255((*C.uchar)(unsafe.Pointer(&output[0])), (*C.uchar)(unsafe.Pointer(&bk.blindingKey[0])), (*C.uchar)(unsafe.Pointer(&input[0])))
	if ret < 0 {
		return nil, errors.New("point not on curve")
	}
	return output, nil
}

func (bk *BlindingKey) Blind(values [][]byte) error {
	sInputs := make([]string, len(values))
	for i, v := range values {
		sInputs[i] = string(v)
	}
	sort.Strings(sInputs)
	for i := 0; i < len(sInputs)-1; i++ {
		if sInputs[i] == sInputs[i+1] {
			return errors.New("inputs are not distinct")
		}
	}
	for i := range values {
		var err error
		values[i], err = bk.exponentiate(values[i])
		if err != nil {
			return err
		}
	}
	bk.permute(values)
	return nil
}

type Blinder struct {
	key *BlindingKey
}

func (b *Blinder) KeyForDay(dayID int) *BlindingKey {
	// TODO: different keys for different days
	return b.key
}

type BlindingRequest struct {
	DayID  int
	Inputs []string
}

type BlindingResponse struct {
	Outputs []string
}

func (b *Blinder) actualServeHTTP(rw http.ResponseWriter, req *http.Request) error {
	if req.Method != http.MethodPost {
		return errors.New("Only POST allowed")
	}

	// TODO: first hash the whole request and compare with signature in authorization
	var r BlindingRequest
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		return err
	}

	tokens := make([][]byte, len(r.Inputs))
	for i, s := range r.Inputs {
		var err error
		tokens[i], err = hex.DecodeString(s)
		if err != nil {
			return err
		}
	}

	if err := b.KeyForDay(r.DayID).Blind(tokens); err != nil {
		return err
	}

	resp := &BlindingResponse{Outputs: make([]string, len(tokens))}

	for i, t := range tokens {
		resp.Outputs[i] = hex.EncodeToString(t)
	}

	response, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write(response)
	return nil
}

func (b *Blinder) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	err := b.actualServeHTTP(rw, req)
	if err != nil {
		log.Print("Request error: ", err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
	}
}

func main() {
	if C.sodium_init() < 0 {
		panic("sodium_init")
	}
	// TODO: this should be read (for each day separately) from secrets storage
	masterKey := "abcdef"
	b := &Blinder{
		key: NewBlindingKey(masterKey),
	}
	http.Handle("/v0/blind", b)
	log.Fatal(http.ListenAndServe(":8787", nil))
}
