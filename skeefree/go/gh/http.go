package gh

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func setupHttpClient() (*http.Client, error) {
	httpTimeout := time.Second
	dialTimeout := func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, httpTimeout)
	}
	httpTransport := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		Dial:                  dialTimeout,
		ResponseHeaderTimeout: httpTimeout,
	}
	httpClient := &http.Client{Transport: httpTransport}

	return httpClient, nil
}
