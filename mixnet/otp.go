package mixnet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgraph-io/ristretto"
	"log"
	"net/http"
)

const cxidLength = 36
const cacheSize = 100 * 1000 * 1000

type OTPChecker struct {
	URL string

	// cache maps from OTP to its assigned cxid
	cache *ristretto.Cache
}

func NewOTPChecker(url string) *OTPChecker {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cacheSize * 10,
		MaxCost:     cacheSize,
		BufferItems: 64,
	})
	if err != nil {
		log.Fatal(err)
	}
	return &OTPChecker{
		URL:   url,
		cache: cache,
	}
}

var ErrAlreadyBound = errors.New("this OTP has already been used on a different phone")
var ErrBadOTP = errors.New("the OTP is invalid")

func (oc *OTPChecker) Check(otp string, cxid string) error {
	if len(cxid) != cxidLength {
		return fmt.Errorf("invalid length of cxid")
	}
	if expectedCxid, ok := oc.cache.Get(otp); ok {
		if cxid != expectedCxid {
			return ErrAlreadyBound
		}
		return nil
	}
	err := oc.check(otp, cxid)
	if err == nil {
		oc.cache.Set(otp, cxid, 1)
	}
	return err
}

func (oc *OTPChecker) check(otp string, cxid string) error {
	type request struct {
		OTP  string
		Cxid string
	}
	var r request
	r.OTP = otp
	r.Cxid = cxid
	reqBody, err := json.Marshal(r)
	if err != nil {
		return err
	}
	resp, err := http.Post(oc.URL, "application/json", bytes.NewReader(reqBody))
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode == 401 {
		return ErrBadOTP
	}
	if resp.StatusCode == 403 {
		return ErrAlreadyBound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("otp validation returned %d: %s", resp.StatusCode, resp.Status)
	}
	return nil
}
