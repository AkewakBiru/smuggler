package smuggler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"strconv"
	"unicode"

	"github.com/rs/zerolog/log"

	"net/http"
	"net/url"
	"os"
	"smuggler/config"
	"smuggler/smuggler/h1"
	"strings"
	"time"
)

// the idea is to disrupt the way http request are dealt with (typically in FIFO), if a user
// sneaks in a request in another request, the synchronization will be affected resulting in
// weird behaviours (users receiving response meant to be received by other users)
type Desyncer interface {
	H1Test(*h1.Payload) (int, error) // returns 1 if connection timedout, 0 if normal response,\
	// testCLTE(*h1.Payload) bool       // 2 if disconnected before timeout
	// testTECL(*h1.Payload) bool
	// runCLTECL() (bool, error) // a wrapper for clte and tecl test
	GetCookie() error
	RunTests() error
	ParseURL(host string) error
}

// Implements the Desyncer interface and has the state of each host that is tested
type DesyncerImpl struct {
	Desyncer

	URL    *url.URL
	Cookie string
	Hdr    map[string]string

	Done chan int
}

func (d *DesyncerImpl) ParseURL(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	d.URL = u
	if len(d.URL.Scheme) == 0 && len(d.URL.Port()) == 0 {
		return errors.New("invalid URL: Empty Scheme & Port")
	}
	if len(d.URL.Scheme) > 0 && d.URL.Scheme != "https" && d.URL.Scheme != "http" {
		return fmt.Errorf("unsupported scheme: %s: valid schemes: http,https", d.URL.Scheme)
	}
	if len(d.URL.Port()) > 0 {
		portInt, err := strconv.Atoi(d.URL.Port())
		if err != nil {
			return fmt.Errorf("%v: error parsing port number", err)
		}
		if portInt >= (1 << 16) {
			return errors.New("invalid port: port must be in range [1-65535]")
		}
	}
	if len(d.URL.Scheme) == 0 {
		if d.URL.Port() == "443" {
			d.URL.Scheme = "https"
		} else {
			d.URL.Scheme = "http"
		}
	}

	if d.URL.Path == "" {
		d.URL.Path = "/"
	}

	if len(d.URL.User.Username()) > 0 {
		d.Hdr["Authorization"] = fmt.Sprintf("Basic %s",
			base64.StdEncoding.EncodeToString([]byte(d.URL.User.String())))
	}
	return nil
}

// builds a new payload
func (d *DesyncerImpl) NewPl(pl string) *h1.Payload {
	payload := h1.Payload{HdrPl: pl, URL: d.URL}
	headers := make(map[string]string)
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"
	headers["Connection"] = "close"
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["Host"] = d.URL.Host // this is causing a big issue // set it to just host if port is 80/443 else host:port

	payload.Header = headers
	for k, v := range d.Hdr {
		payload.Header[k] = v
	}
	if len(d.Cookie) > 0 {
		payload.Header["Cookie"] = d.Cookie
	}
	return &payload
}

// use Go's http client, because it follows redirects
func (d *DesyncerImpl) GetCookie() error {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return http.ErrUseLastResponse
			}
			if len(via) > 0 {
				req.Method = via[0].Method
			}
			return nil
		},
		Timeout: config.Glob.Timeout,
	}

	var resp *http.Response
	var err error
	switch config.Glob.Method {
	case http.MethodPost:
		resp, err = client.Post(d.URL.String(), "", nil)
	case http.MethodGet:
		resp, err = client.Get(d.URL.String())
	case http.MethodHead:
		resp, err = client.Head(d.URL.String())
	default:
		return errors.New("HTTP: unsupported method: options [GET, POST, HEAD]")
	}
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("invalid endpoint: endpoint returned %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if strings.TrimRight(d.URL.String(), "/") != strings.TrimRight(resp.Request.URL.String(), "/") {
		d.URL = resp.Request.URL // incase of a redirect, update the URL
	}

	var hdr []string
	hdr = resp.Header.Values("Set-Cookie")
	if hdr == nil {
		hdr = resp.Header.Values("set-cookie")
	}
	var res []string = make([]string, len(hdr))
	for i, v := range hdr {
		if idx := strings.Index(v, ";"); idx != -1 {
			res[i] = v[:idx]
		}
	}
	d.Cookie = strings.Join(res, "; ")
	return nil
}

func (d *DesyncerImpl) RunTests() {
	cl := CL{DesyncerImpl: d}
	te := TE{DesyncerImpl: d}

	go cl.Run()
	go te.Run()

	<-d.Done
}

func (d *DesyncerImpl) H1Test(p *h1.Payload) (int, error) {
	t := h1.Transport{}
	q := d.URL.Query()
	q.Set("t", fmt.Sprintf("%d", rand.Int32N(math.MaxInt32))) // avoid caching
	d.URL.RawQuery = q.Encode()
	start := time.Now()
	resp, err := t.RoundTrip(&h1.Request{Url: d.URL, Payload: p, Timeout: config.Glob.Timeout})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Compare(err.Error(), "read timeout") == 0 {
			return 1, err // deadline exceeds after waiting for 'timeout' seconds
		}
		return -1, err
	}
	diff := time.Since(start)
	defer resp.Body.Close()

	var sample []byte = make([]byte, 100)
	if _, err = resp.Body.Read(sample); err != nil && err != io.EOF {
		return -1, fmt.Errorf("socket error: %v", err)
	}
	if len(sample) == 0 {
		if diff < time.Duration(config.Glob.Timeout-time.Second) {
			return 2, nil // disconnected before timeout
		}
		return 1, nil // connection timeout (probably)
	}
	return 0, nil // normal response
}

func (d *DesyncerImpl) GenReport(p *h1.Payload, t time.Duration) {
	if err := createDir("/result/"); err != nil {
		log.Warn().Err(err).Msg("")
	}
	if err := createDir(fmt.Sprintf("/result/%s", d.URL.Hostname())); err != nil {
		log.Warn().Err(err).Msg("")
	}
	pwd, err := os.Getwd()
	if err != nil {
		log.Warn().Err(err).Msg("")
		return
	}
	fname := fmt.Sprintf("%s/result/%s/%ss", pwd, d.URL.Hostname(), d.URL.Query().Get("t"))
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn().Err(err).Msg("")
		return
	}
	defer file.Close()

	nHdrPl := ""
	for i := 0; i < len(p.HdrPl); i++ {
		if unicode.IsPrint(rune(p.HdrPl[i])) {
			nHdrPl += string(p.HdrPl[i])
		} else {
			nHdrPl += fmt.Sprintf("\\x%02X", p.HdrPl[i])
		}
	}
	p.HdrPl = nHdrPl
	file.WriteString(p.ToString())
}

func createDir(dir string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	info, err := os.Stat(pwd + dir)
	flag := true
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		flag = false
	}
	if flag {
		if !info.IsDir() {
			if err := os.Mkdir(pwd+dir, 0777); err != nil {
				return err
			}
		}
	} else {
		if err := os.Mkdir(pwd+dir, 0777); err != nil {
			return err
		}
	}
	return nil
}
