package h2

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

type Mode byte

const (
	H2 Mode = iota
	H2C
)

// use verbose logging to log all frames
// export GODEBUG=http2debug=2

type clientConn struct {
	framer *http2.Framer
	muBw   sync.Mutex
	bw     *bufio.Writer
	br     *bufio.Reader

	hbuf bytes.Buffer
	henc *hpack.Encoder
	hdec *hpack.Decoder

	nextRes http.Header

	mu           sync.Mutex
	streams      map[uint32]*cstream
	nextStreamID uint32

	maxFrameSize    uint32
	maxDynTableSize uint32

	status  int
	request *http.Request

	readerDone chan struct{}

	readerError error
}

type Transport struct{}

type cstream struct { // adding window would be a good idea but i don't care
	ID   uint32
	resc chan *http.Response
	pr   *PipeReader
	pw   *PipeWriter
}

// used for all payload construction
// Joining payload key and value doesn't add a space for H1
// For H2, i will strip the first space that i find, might mess some payloads
type Payload struct {
	Key string
	Val string
}

type Request struct {
	URL    *url.URL
	Method string
	Hdrs   map[string][]string
	Body   []byte
	Mode   Mode // is it h2/h2c

	Payload *Payload // send it as a header as-is (in lowercase) // strip the first space if found (in value)
}

func BuildReq(req *Request) *http.Request {
	return &http.Request{Method: req.Method,
		URL:        req.URL,
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     req.Hdrs,
		Body:       io.NopCloser(bytes.NewBuffer(req.Body)),
	}
}

func BuildH2CPayload(req *Request) (string, error) {
	final := fmt.Sprintf("%s %s HTTP/1.1\r\n", req.Method, req.URL.Path)
	for k, vv := range req.Hdrs {
		for _, v := range vv {
			final += fmt.Sprintf("%s: %s\r\n", k, v)
		}
	}
	if len(req.Hdrs["Host"]) == 0 {
		final += fmt.Sprintf("Host: %s\r\n", req.URL.Host)
	}
	buf := bytes.NewBuffer(nil)
	f := http2.NewFramer(buf, buf)

	if err := f.WriteSettings(
		http2.Setting{ID: http2.SettingInitialWindowSize, Val: 1 << 15},
		http2.Setting{ID: http2.SettingEnablePush, Val: 0},
		http2.Setting{ID: http2.SettingMaxConcurrentStreams, Val: 100},
	); err != nil {
		return "", err
	}
	str := base64.RawStdEncoding.EncodeToString(buf.Bytes())
	final += "Connection: Upgrade, HTTP2-Settings\r\nUpgrade: h2c\r\nHTTP2-Settings: " + str + "\r\n"
	if req.Payload != nil {
		if req.Payload.Val[0] == ' ' {
			final += fmt.Sprintf("%s:%s", req.Payload.Key, req.Payload.Val[1:])
		} else {
			final += fmt.Sprintf("%s:%s", req.Payload.Key, req.Payload.Val)
		}
	}
	final += "\r\n"
	if len(req.Body) > 0 {
		final += string(req.Body)
	}
	return final, nil
}

// use my own request header
func (t Transport) RoundTrip(req *Request) (*http.Response, error) {
	if req.URL.Scheme == "https" && req.Mode == H2C {
		return nil, errors.New("h2c: unsupported scheme") // h2 cleartext
	}

	host, port, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		host = req.URL.Host
		if req.Mode == H2C {
			port = "80"
		} else {
			port = "443"
		}
	}

	cfg := tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}

	var conn net.Conn
	if req.Mode == H2C {
		conn, err = net.Dial("tcp", host+":"+port)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	} else {
		conn, err = tls.Dial("tcp", host+":"+port, &cfg)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		if err := conn.(*tls.Conn).Handshake(); err != nil {
			return nil, err
		}
	}

	if err := conn.SetReadDeadline(time.Now().Add(time.Second * 5)); err != nil {
		return nil, err
	}

	client := clientConn{
		bw: bufio.NewWriter(conn),
		br: bufio.NewReader(conn),

		nextStreamID: 1,

		streams:    make(map[uint32]*cstream),
		readerDone: make(chan struct{}),

		maxFrameSize: 1 << 14,

		maxDynTableSize: 4096,

		request: BuildReq(req),
	}

	if req.Mode == H2C {
		p, err := BuildH2CPayload(req)
		if err != nil {
			return nil, err
		}
		if _, err := conn.Write([]byte(p)); err != nil {
			return nil, err
		}
		res := make([]byte, 1024)
		if _, err := conn.Read(res); err != nil {
			return nil, err
		}
		if !strings.Contains(string(res), "HTTP/1.1 101 Switching Protocols") {
			return nil, fmt.Errorf("h2c is not supported: %s", req.URL.String())
		}
	}

	client.henc = hpack.NewEncoder(&client.hbuf)
	client.hdec = hpack.NewDecoder(client.maxDynTableSize, client.onNewHeaderField)
	client.nextRes = make(http.Header)

	client.framer = http2.NewFramer(client.bw, client.br)
	if _, err := client.bw.Write([]byte(http2.ClientPreface)); err != nil {
		return nil, fmt.Errorf("error Sending Preface: %v", err)
	}

	if req.Mode == H2 {
		if err := client.framer.WriteSettings(
			http2.Setting{ID: http2.SettingInitialWindowSize, Val: 1 << 15},
			http2.Setting{ID: http2.SettingEnablePush, Val: 0},
			http2.Setting{ID: http2.SettingMaxConcurrentStreams, Val: 100}); err != nil {
			return nil, fmt.Errorf("error sending HTTP/2 Settings frame: %v", err)
		}

		if err := client.framer.WriteWindowUpdate(0, 1<<15); err != nil {
			return nil, fmt.Errorf("error sending HTTP/2 WINDOW_UPDATE frame: %v", err)
		}
		client.bw.Flush()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6)
	defer cancel()

	var hasbody bool
	if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions ||
		req.Method == http.MethodTrace || req.Body == nil {
		hasbody = false
	}

	var body []byte = req.Body
	hasbody = len(body) > 0

	go client.readLoop()

	cs := client.Stream()
	hdrs := client.encodeHeaders(req)
	first := true

	endHeaders := true
	endStream := !hasbody
	if req.Mode == H2C {
		body = []byte{}
		hdrs = []byte{}
	}

	for len(hdrs) > 0 { // send header
		chunk := hdrs
		if len(chunk) > int(client.maxFrameSize) {
			client.muBw.Lock()
			chunk = chunk[:client.maxFrameSize]
			client.muBw.Unlock()
		}
		hdrs = hdrs[len(chunk):]
		endHeaders = len(hdrs) == 0
		if first {
			hf := http2.HeadersFrameParam{
				StreamID:      cs.ID,
				EndHeaders:    endHeaders,
				BlockFragment: chunk,
				EndStream:     endStream,
			}
			client.muBw.Lock()
			if err := client.framer.WriteHeaders(hf); err != nil {
				client.readerError = err
				client.muBw.Unlock()
				return nil, err
			}
			client.bw.Flush()
			client.muBw.Unlock()
		} else {
			client.muBw.Lock()
			if err := client.framer.WriteContinuation(cs.ID, endHeaders, chunk); err != nil {
				client.readerError = err
				client.muBw.Unlock()
				return nil, err
			}
			client.bw.Flush()
			client.muBw.Unlock()
		}
	}
	// send DATA
	if err := client.writeData(cs, body); err != nil {
		client.muBw.Lock()
		client.readerError = err
		client.muBw.Unlock()
	}

	select {
	case res := <-cs.resc:
		return res, client.readerError
	case <-client.readerDone:
		return nil, client.readerError
	case <-ctx.Done():
		return nil, errors.New("request timed out")
	}
}

func (c *clientConn) writeData(cs *cstream, body []byte) error {
	endStream := false
	for len(body) > 0 { // send header
		chunk := body
		if len(chunk) > int(c.maxFrameSize) {
			chunk = chunk[:c.maxFrameSize]
		} else {
			endStream = true
		}
		body = body[len(chunk):]
		c.muBw.Lock()
		if err := c.framer.WriteData(cs.ID, endStream, chunk); err != nil {
			c.muBw.Unlock()
			return err
		}
		c.bw.Flush()
		c.muBw.Unlock()
	}
	return nil
}

func (c *clientConn) readLoop() {
	defer close(c.readerDone)

	for {
		fr, err := c.framer.ReadFrame()
		if err != nil {
			c.readerError = err
			return
		}

		cs := c.streamByID(fr.Header().StreamID)
		streamEnded := false
		switch f := fr.(type) {
		case *http2.HeadersFrame:
			cs.pr, cs.pw = NewP(nil)
			c.hdec.Write(f.HeaderBlockFragment())
			streamEnded = f.StreamEnded()
		case *http2.ContinuationFrame:
			c.hdec.Write(f.HeaderBlockFragment())
		case *http2.SettingsFrame:
			if err := c.SettingsFrameHandler(f); err != nil {
				c.muBw.Lock()
				c.readerError = err
				c.muBw.Unlock()
				return
			}
		case *http2.DataFrame:
			streamEnded = f.StreamEnded()
			data := f.Data()
			size := len(data)
			if _, err := cs.pw.Write(data); err != nil {
				log.Print(err)
			}
			if !streamEnded && size > 1024 {
				c.muBw.Lock()
				// i am not sure if i even need to send this one as my conn window size is 1 << 31 - 1 bytes ~ 2 Gb
				if err := c.framer.WriteWindowUpdate(0, uint32(size)); err != nil {
					log.Print(err)
				}
				if err := c.framer.WriteWindowUpdate(cs.ID, uint32(size)); err != nil {
					log.Print(err)
				}
				c.bw.Flush()
				c.muBw.Unlock()
			}
		case *http2.RSTStreamFrame:
			if f.ErrCode != 0 {
				c.readerError = fmt.Errorf("error occurred with code: %s", f.ErrCode.String())
			}
			return
		case *http2.GoAwayFrame:
			return
		case *http2.UnknownFrame:
			log.Print("UNKNOWN frame received")
		default:
		}
		if streamEnded {
			if cs != nil {
				cs.pw.Close()
			}
			if cs == nil {
				c.readerError = errors.New("couldn't find stream")
				return
			} else {
				cs.resc <- &http.Response{
					Header:     c.nextRes,
					StatusCode: c.status,
					Status:     http.StatusText(c.status),
					Proto:      "HTTP/2.0",
					ProtoMajor: 2,
					ProtoMinor: 0,
					Body:       cs.pr,
					Request:    c.request,
				}
			}
		}
	}
}

func (c *clientConn) SettingsFrameHandler(f *http2.SettingsFrame) error {
	f.ForeachSetting(func(s http2.Setting) error {
		if http2.SettingMaxFrameSize == s.ID {
			c.muBw.Lock()
			c.maxFrameSize = s.Val
			c.muBw.Unlock()
		}
		return nil
	})
	if !f.IsAck() {
		c.muBw.Lock()
		if err := c.framer.WriteSettingsAck(); err != nil {
			c.muBw.Unlock()
			return err
		}
		c.bw.Flush()
		c.muBw.Unlock()
	}
	return nil
}

func (c *clientConn) Stream() *cstream {
	c.mu.Lock()
	defer c.mu.Unlock()

	cs := &cstream{
		ID:   c.nextStreamID,
		resc: make(chan *http.Response, 1),
	}
	c.nextStreamID += 2
	c.streams[cs.ID] = cs
	return cs
}

func (c *clientConn) streamByID(id uint32) *cstream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streams[id]
}

func (c *clientConn) encodeHeaders(req *Request) []byte {
	c.writeHeader(":authority", req.URL.Host)
	c.writeHeader(":method", req.Method)
	if len(req.URL.Path) == 0 {
		c.writeHeader(":path", "/")
	} else {
		c.writeHeader(":path", req.URL.Path)
	}
	c.writeHeader(":scheme", "https")
	for k, vv := range req.Hdrs {
		for _, v := range vv {
			c.writeHeader(strings.ToLower(k), v)
		}
	}
	if req.Payload != nil && len(req.Payload.Key) > 0 {
		if req.Payload.Val[0] == ' ' {
			c.writeHeader(strings.ToLower(req.Payload.Key), req.Payload.Val[1:])
		} else {
			c.writeHeader(strings.ToLower(req.Payload.Key), req.Payload.Val)
		}
	}
	return c.hbuf.Bytes()
}

func (c *clientConn) writeHeader(name, val string) {
	c.henc.WriteField(hpack.HeaderField{Name: name, Value: val})
}

func (c *clientConn) onNewHeaderField(f hpack.HeaderField) {
	if f.Name == ":status" {
		code, err := strconv.Atoi(f.Value)
		if err != nil {
			log.Print(err)
		} else {
			c.status = code
		}
	}
	c.nextRes.Add(f.Name, f.Value)
}

func GetRequestSummary(req *Request) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s HTTP/2\r\n", req.Method, req.URL.EscapedPath()))
	for k, vv := range req.Hdrs {
		for _, v := range vv {
			sb.WriteString(fmt.Sprintf("%s:%s\r\n", k, v))
		}
	}

	sb.WriteString(fmt.Sprintf("Host: %s\r\n", req.URL.Host))
	if req.Payload != nil {
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", req.Payload.Key, req.Payload.Val))
	}

	sb.WriteString("\r\n")
	if len(req.Body) > 0 {
		sb.WriteString(string(req.Body))
	}

	return sb.String()
}
