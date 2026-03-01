package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func main() {
	server := &http.Server{
		Addr:         ":8080",
		Handler:      http.HandlerFunc(proxyHandler),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Server Started...")
	log.Fatal(server.ListenAndServe())
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		handleHTTPS(w, r)
		return
	}
	handleHTTP(w, r)
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("HTTPS %s %s -> %s", r.Method, r.URL.String(), r.Host)

	// When the client uses HTTPs, proxy would not read
	// anything if the client sent encrypted data. As such,
	// the client requests the proxy to start a tunnel with
	// method CONNECT (specifically designed for such task)
	// After opening a TCP connection with remote destination,
	// just relay TCP incoming data in both directions

	host := r.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	server, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	//
	// Use Hijack when you don't want to use built-in server's
	// implementation of the HTTP protocol. We use Hijack to
	// access raw TCP socket in order to transfer raw bytes
	// and communicates
	//

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		server.Close()
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		server.Close()
		return
	}

	// Send tunnel established
	_, err = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		client.Close()
		server.Close()
		return
	}

	// Bidirectional copy
	go transfer(server, client)
	go transfer(client, server)
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {

	// When the client uses HTTP, proxy can read anything.
	// As such, proxy reads remote destination, open a TCP
	// connection and simply replicates messages the client
	// is sending
	// Theoritically, the proxy can also mangle requests,
	// but the following implementation acts as a simple
	// mirror

	log.Printf("HTTP %s %s -> %s", r.Method, r.URL.String(), r.Host)

	host := r.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}

	server, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijack not supported", http.StatusInternalServerError)
		server.Close()
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		server.Close()
		return
	}

	err = r.Write(server)
	if err != nil {
		client.Close()
		server.Close()
		return
	}

	io.Copy(client, server)
}

// Helpers

func transfer(dst net.Conn, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
