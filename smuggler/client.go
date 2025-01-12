package smuggler

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Request struct {
	Url     *url.URL
	Payload *Payload
}

type clientConn struct {
	conn net.Conn

	readCh    chan struct{} // closed on error
	readError error
	resp      chan *http.Response

	req *Request
}

type Transport struct{}

func (t *Transport) RoundTrip(req *Request) (*http.Response, error) {
	cc := clientConn{}
	if req.Url == nil || req.Url.Host == "" {
		return nil, errors.New("invalid URL")
	}

	host, port, err := net.SplitHostPort(req.Url.Host)
	if err != nil {
		host = req.Url.Host
		port = "443"
		req.Url.Scheme = "https"
	}

	dialer := net.Dialer{Timeout: time.Millisecond * 2000}
	if req.Url.Scheme == "https" {
		cfg := &tls.Config{InsecureSkipVerify: true,
			NextProtos: []string{"http/1.1"}}
		conn, err := tls.DialWithDialer(&dialer, "tcp", fmt.Sprintf("%s:%s", host, port), cfg)
		if err != nil {
			return nil, err
		}
		cc.conn = conn
	} else {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", host, port), time.Second*2)
		if err != nil {
			return nil, err
		}
		cc.conn = conn
	}
	defer cc.conn.Close()

	cc.req = req
	cc.resp = make(chan *http.Response, 1)
	cc.readCh = make(chan struct{}, 1)

	if _, err := cc.conn.Write([]byte(req.Payload.ToString())); err != nil {
		return &http.Response{ContentLength: -1}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	go cc.readResponse()

	select {
	case <-ctx.Done():
		return &http.Response{ContentLength: 1}, errors.New("read context deadline exceeded")
	case <-cc.readCh:
		return nil, cc.readError
	case resp := <-cc.resp:
		return resp, nil
	}
}

func (c *clientConn) readResponse() {
	defer close(c.readCh)

	r := bufio.NewReader(c.conn)
	if resp, err := http.ReadResponse(r, nil); err != nil {
		c.readError = err
	} else {
		c.resp <- resp
	}
}
