package configs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func fetchHTTP(url string) ([]byte, error) {
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("got status code %d (%s) from %s", resp.StatusCode, resp.Status, url)
	}
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func LoadConfig(path string, config interface{}) error {
	var configText []byte
	var err error
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		configText, err = fetchHTTP(path)
	} else {
		configText, err = ioutil.ReadFile(path)
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(configText, config); err != nil {
		return err
	}
	return nil
}
