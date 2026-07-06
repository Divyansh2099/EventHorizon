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
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	
	resp, err := client.Get("https://127.0.0.1:8082/")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	
	fmt.Println("Status:", resp.Status)
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Body length: %d\n", len(body))
}
