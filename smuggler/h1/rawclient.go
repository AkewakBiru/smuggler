package h1

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
)

type RawClient struct {
	conn net.Conn
}

func NewClient(url *url.URL) (*RawClient, error) {
	if url == nil || url.Host == "" {
		return nil, errors.New("invalid URL")
	}

	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		host = url.Host
		if url.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	client := RawClient{}
	if url.Scheme == "https" {
		f, err := os.OpenFile("/Users/akewakbiru/Desktop/sslkeys.log", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		cfg := &tls.Config{InsecureSkipVerify: false, NextProtos: []string{"http/1.1"}, KeyLogWriter: f}
		client.conn, err = tls.Dial("tcp", host+":"+port, cfg)
		if err != nil {
			return nil, err
		}
		if err = client.conn.(*tls.Conn).Handshake(); err != nil {
			return nil, err
		}
		return &client, nil
	}
	client.conn, err = net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *RawClient) readResponse() (string, error) {
	b := make([]byte, 2048)
	_, err := r.conn.Read(b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *RawClient) SendPipelinedRequests(reqs ...string) string {
	go func() {
		for _, req := range reqs {
			if _, err := r.conn.Write([]byte(req)); err != nil {
				log.Print(err)
			}
		}
	}()

	var sb strings.Builder
	for i := range len(reqs) {
		_ = i
		str, err := r.readResponse()
		if err != nil {
			log.Print(err)
			return sb.String()
		}
		sb.WriteString(str)
	}
	return sb.String()
}

func (r *RawClient) Close() {
	r.conn.Close()
}
