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
		// f, err := os.OpenFile("/Desktop/sslkeys.log", os.O_APPEND|os.O_RDWR, 0644)
		// if err != nil {
		// 	panic(err)
		// }
		cfg := &tls.Config{InsecureSkipVerify: true,
			// KeyLogWriter: f,
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
	// return
	// }

	// resBuf := bytes.NewBuffer(nil)
	// res := make([]byte, 1024)
	// for {
	// 	s, err := c.conn.Read(res)
	// 	if err != nil {
	// 		if err == io.EOF {
	// 			resBuf.Write(res[:s])
	// 			break
	// 		} else {
	// 			c.readError = err
	// 			return
	// 		}
	// 	}
	// 	resBuf.Write(res[:s])
	// 	if s == 0 || s < 1<<15 {
	// 		break
	// 	}
	// 	memset(res)
	// }

	// tot := resBuf.String()
	// c.resp <- ParseResp(tot)
}

// func ParseRespFirstLine(line string) [3]int {
// 	var res [3]int
// 	resCode := strings.Split(line, " ")
// 	if len(resCode) == 3 {
// 		idx := strings.Index(resCode[0], ".")
// 		if idx != -1 {
// 			fr, err := strconv.Atoi(resCode[0][idx-1 : idx])
// 			if err == nil {
// 				res[0] = fr
// 			}
// 			sc, err := strconv.Atoi(resCode[0][idx+1 : idx+2])
// 			if err == nil {
// 				res[1] = sc
// 			}
// 		}
// 		th, err := strconv.Atoi(strings.TrimSpace(resCode[1]))
// 		if err == nil {
// 			res[2] = th
// 		}
// 	}
// 	return res
// }

// func ParseResp(str string) *http.Response {
// 	idx := strings.Index(str, "\r\n\r\n")
// 	resBuf := bytes.NewBuffer([]byte(str[idx+4:]))
// 	resp := http.Response{Body: io.NopCloser(resBuf)}

// 	if idx != -1 {
// 		str = str[:idx]
// 	}
// 	if i := strings.Index(str, "\r\n"); i != -1 {
// 		firstline := str[:i]
// 		a := ParseRespFirstLine(firstline)
// 		resp.ProtoMajor = a[0]
// 		resp.ProtoMinor = a[1]
// 		resp.StatusCode = a[2]
// 		resp.Status = http.StatusText(resp.StatusCode)
// 	}
// 	res := make(map[string][]string)

// 	strV := strings.Split(str, "\r\n")
// 	for _, s := range strV {
// 		if idx := strings.Index(s, ":"); idx != -1 {
// 			key := strings.TrimSpace(s[:idx])
// 			res[key] = append(res[key], strings.TrimSpace(s[idx+1:]))
// 		}
// 	}

// 	resp.Header = res
// 	return &resp
// }
