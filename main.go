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

	// The HTTP CONNECT method is used to create an HTTP
	// tunnel through a proxy server. By sending an HTTP
	// CONNECT request, the client asks the proxy server
	// to forward the TCP connection

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
