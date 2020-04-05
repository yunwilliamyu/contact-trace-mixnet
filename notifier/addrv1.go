package notifier

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"golang.org/x/crypto/nacl/box"
)

func (ps *PollServer) unsealAddressV1(addr []byte) (hint uint16, deaddropID DeadDropID, err error) {
	// check len?
	decAddr, ok := box.OpenAnonymous(nil, addr, &ps.conf.PublicKey, &ps.privateKey)
	if !ok {
		return hint, deaddropID, fmt.Errorf("cannot decrypt address")
	}
	if got, want := len(decAddr), IDSize+2; got != want {
		return hint, deaddropID, fmt.Errorf("invalid address length: %d, expected %d", got, want)
	}
	copy(deaddropID[:], decAddr[:IDSize])
	hint = binary.LittleEndian.Uint16(decAddr[IDSize:])
	return hint, deaddropID, nil
}

func (dcl *DeadDropClient) MakeAddressV1(id uint16) []byte {
	var rawAddress [IDSize + 2]byte
	copy(rawAddress[:IDSize], dcl.address[:])
	binary.LittleEndian.PutUint16(rawAddress[IDSize:], id)
	encAddress, err := box.SealAnonymous(nil, rawAddress[:], &dcl.conf.PublicKey, cryptorand.Reader)
	if err != nil {
		panic(err)
	}
	return encAddress
}
