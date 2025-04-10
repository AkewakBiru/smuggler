package smuggler

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"smuggler/config"
	"smuggler/smuggler/h1"
	"smuggler/utils"
	"strings"
	"time"
)

// the idea is to disrupt the way http request are dealt with (typically in FIFO), if a user
// sneaks in a request in another request, the synchronization will be affected resulting in
// weird behaviours (users receiving response meant to be received by other users)
type Desyncer interface {
	H1Test(*h1.Payload) (int, error)
	GetCookie() error
	getCookie(bool) error
	RunTests() error
	ParseURL(host string) error
}

// Implements the Desyncer interface and has the state of each host that is tested
type DesyncerImpl struct {
	Desyncer

	H1Supported bool
	H2Supported bool

	URL    *url.URL
	Body   string
	Method string

	Hdr map[string][]string

	TestDone chan struct{} // closed on success, if exit-on-success is set

	Wg     sync.WaitGroup
	Ctx    context.Context
	Cancel context.CancelFunc
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
		d.Hdr["Authorization"] = []string{fmt.Sprintf("Basic %s",
			base64.StdEncoding.EncodeToString([]byte(d.URL.User.String())))}
	}
	return nil
}

// builds a new payload
func (d *DesyncerImpl) NewPl(pl string) *h1.Payload {
	payload := h1.Payload{HdrPl: pl, URL: *d.URL, Method: d.Method}
	headers := make(map[string]string)
	// headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["Host"] = d.URL.Host         // this is causing a big issue // set it to just host if port is 80/443 else host:port
	for k, vv := range config.Glob.Hdr { // headers included in all requests
		headers[k] = vv[0]
	}
	payload.Header = headers
	for k, v := range d.Hdr { // per-host headers
		payload.Header[k] = strings.Join(v, "; ") // applies to cookies only for now
	}

	if len(d.Hdr["Cookie"]) > 0 {
		payload.Header["Cookie"] = strings.Join(d.Hdr["Cookie"], "; ")
	}
	return &payload
}

func (d *DesyncerImpl) GetCookie() error {
	err := d.getCookie(true)
	if err == nil {
		d.H2Supported = true
	}

	err2 := d.getCookie(false)
	if err2 == nil {
		d.H1Supported = true
	}

	if err != nil && err2 != nil {
		return err
	}
	return nil
}

// some sites start with h1.1, then after redirect, upgrade to h2 (disallow h1.1)
// use Go's http client, because it follows redirects
func (d *DesyncerImpl) getCookie(forceH2 bool) error {
	t := &http.Transport{
		ForceAttemptHTTP2: forceH2,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Jar:       jar,
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return http.ErrUseLastResponse
			}
			if len(via) > 0 {
				req.Method = via[0].Method
			}
			return nil
		},
		Timeout: time.Second * 5, // wait for 5 seconds for a response
	}

	req, _ := http.NewRequest(d.Method, d.URL.String(), nil)
	for k, vv := range config.Glob.Hdr {
		req.Header[k] = append(req.Header[k], vv...)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("invalid endpoint: endpoint returned %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if strings.TrimRight(d.URL.String(), "/") != strings.TrimRight(resp.Request.URL.String(), "/") {
		d.URL = resp.Request.URL // incase of a redirect, update the URL
	}

	if len(d.Hdr["Cookie"]) == 0 {
		d.Hdr["Cookie"] = []string{}
	}
	for _, j := range jar.Cookies(d.URL) {
		cv := fmt.Sprintf("%s=%s", j.Name, j.Value)
		if utils.KeyExists(d.Hdr["Cookie"], j.Name) {
			continue
		}
		if !utils.ValueExists(d.Hdr["Cookie"], cv) {
			d.Hdr["Cookie"] = append(d.Hdr["Cookie"], cv)
		}
	}
	return nil
}

// tests run concurrently
func (d *DesyncerImpl) runTestsC() {
	cl := CL{DesyncerImpl: d}
	te := TE{DesyncerImpl: d}
	h2 := H2{DesyncerImpl: d}

	d.Wg.Add(3) //increase delta when more tests are added
	go cl.Run()
	go te.Run()
	go h2.Run()

	go func() {
		d.Wg.Wait()
		close(d.TestDone) // wait for all and close the channel
	}()

	select {
	case <-d.TestDone: // signaled when one test is done, only signaled when exit on success is set
		d.Cancel()
	case <-d.Ctx.Done(): // signaled when all tests are complete
	}
}

// tests run sequentially
func (d *DesyncerImpl) runTestsN() {
	cl := CL{DesyncerImpl: d}
	te := TE{DesyncerImpl: d}
	h2 := H2{DesyncerImpl: d}

	tests := map[config.Priority][]func() bool{
		config.CLTEH2: {cl.Run, te.Run, h2.Run},
		config.CLH2TE: {cl.Run, h2.Run, te.Run},
		config.H2TECL: {h2.Run, te.Run, cl.Run},
		config.H2CLTE: {h2.Run, cl.Run, te.Run},
		config.TECLH2: {te.Run, cl.Run, h2.Run},
		config.TEH2CL: {te.Run, h2.Run, cl.Run},
	}

	for _, testFunc := range tests[config.Glob.Priority] {
		if testFunc() {
			return
		}
	}
}

func (d *DesyncerImpl) RunTests() {
	if config.Glob.Concurrent {
		d.runTestsC()
		return
	}
	d.runTestsN()
}

func (d *DesyncerImpl) H1Test(p *h1.Payload) (int, error) {
	t := h1.Transport{}
	p.URL = *d.URL
	q := p.URL.Query()
	q.Set("t", fmt.Sprintf("%d", rand.Int32N(math.MaxInt32))) // avoid caching
	p.URL.RawQuery = q.Encode()
	start := time.Now()
	resp, err := t.RoundTrip(&h1.Request{Url: &p.URL, Payload: p, Timeout: config.Glob.Timeout})
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

func (d *DesyncerImpl) GenReport(p *h1.Payload) {
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
	fname := fmt.Sprintf("%s/result/%s/%ss", pwd, d.URL.Hostname(), p.URL.Query().Get("t"))
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn().Err(err).Msg("")
		return
	}
	defer file.Close()

	p.HdrPl = utils.HexEscapeNonPrintable(p.HdrPl)
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
