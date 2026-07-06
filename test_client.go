//go:build ignore

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
)

func main() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MaxVersion:         tls.VersionTLS12,
			MinVersion:         tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("https://127.0.0.1:8082/static")
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", body)
}
