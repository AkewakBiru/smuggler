package smuggler

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"
)

type Glob struct {
	Cookie    string
	Method    string
	URL       *url.URL
	Header    map[string]string
	ExitEarly bool
	Timeout   time.Duration
	File      string
}

var glob Glob

// the idea is to disrupt the way http request are dealt with (typically in FIFO), if a user
// sneaks in a request in another request, the synchronization will be affected resulting in
// weird behaviours (users receiving response meant to be received by other users)
type Desyncer interface {
	test(*Payload) (int, error)  // returns 1 if connection timedout, 0 if normal response,\
	testCLTE(int, *Payload) bool // 2 if disconnected before timeout
	testTECL(int, *Payload) bool
	GetCookie() error
	Start() error
}

type DesyncerImpl struct {
	Desyncer
}

// builds a new payload
func NewPl(pl string) *Payload {
	payload := Payload{HdrPl: pl}
	payload.ReqLine = ReqLine{
		Method:  glob.Method,
		Path:    glob.URL.Path,
		Version: "HTTP/1.1",
		Query:   fmt.Sprintf("q=%d", rand.Int63n(math.MaxInt64))}

	headers := make(map[string]string)
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"
	headers["Connection"] = "Keep-alive"
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["Host"] = glob.URL.Host

	payload.Header = headers
	for k, v := range glob.Header {
		payload.Header[k] = v
	}
	payload.Header["Cookie"] = glob.Cookie
	return &payload
}

func (DesyncerImpl) GetCookie(g *Glob) error {
	glob = *g
	t := Transport{}

	headers := make(map[string]string)
	headers["Host"] = glob.URL.Host
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"
	headers["Cache-Control"] = "no-store"
	headers["Pragma"] = "no-cache"
	headers["Accept"] = "*/*"
	// populate the global headers
	for k, v := range glob.Header {
		headers[k] = v
	}
	payload := Payload{Host: glob.URL.Host, Port: glob.URL.Port(), Header: headers,
		ReqLine: ReqLine{Method: "HEAD",
			Path: glob.URL.Path, Query: fmt.Sprint("nocache=", rand.Int63n(math.MaxInt64)), Version: "HTTP/1.1"}}

	resp, err := t.RoundTrip(&Request{Payload: &payload, Url: glob.URL, Timeout: glob.Timeout})
	if err != nil {
		return err
	}
	resp.Body.Close()

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
	glob.Cookie = strings.Join(res, "; ")
	return nil
}

func (d DesyncerImpl) Start() error {
	f, err := os.OpenFile("smuggler/tests/"+glob.File, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		tmp, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			return err
		}
		payload := NewPl(string(tmp))
		if d.testCLTE(payload) || d.testTECL(payload) {
			return nil
		}
	}
	return nil
}

func (d DesyncerImpl) test(p *Payload) (int, error) {
	t := Transport{}
	start := time.Now()
	resp, err := t.RoundTrip(&Request{Url: glob.URL, Payload: p, Timeout: glob.Timeout})
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
		return -1, fmt.Errorf("error while reading response received:%v", err)
	}
	if len(sample) == 0 {
		if diff < time.Duration(glob.Timeout-time.Second) {
			return 2, nil // disconnected before timeout
		}
		return 1, nil // connection timeout (probably)
	}
	return 0, nil // normal response
}

func (d DesyncerImpl) testTECL(p *Payload) bool {
	log.Print("Testing TECL...")
	p.Body = "0\r\n\r\nG"

	ctr := 0
	for {
		p.Cl = 6

		start := time.Now()
		ret, err := d.test(p)
		if ret != 1 {
			if ret == -1 {
				log.Printf("Socket error: %v", err)
			} else if ret == 0 {
				log.Print("No issues found")
			} else if ret == 2 {
				log.Printf("DISCONNECTED: %v", err)
			}
			return false
		}
		diff := time.Since(start)
		p.Cl = 5
		ret2, err := d.test(p)
		if ret2 == -1 {
			log.Print(err)
			return false
		}
		if ret2 == 0 {
			ctr++
			if ctr < 3 {
				continue
			} else {
				log.Printf("Potential TECL issue found - %s@%s://%s%s", (*p).ReqLine.Method,
					glob.URL.Scheme, glob.URL.Host, glob.URL.Path)
				GenReport(p, diff)
				return glob.ExitEarly
			}
		} else {
			log.Print("TECL timeout on both length 5 and 6")
			return false
		}
	}
}

func (d DesyncerImpl) testCLTE(p *Payload) bool {
	log.Print("Testing CLTE...")
	p.Body = fmt.Sprintf("%X\r\nG\r\n0\r\n\r\n", 1)

	ctr := 0
	for {
		p.Cl = 4
		start := time.Now()
		ret, err := d.test(p)
		if ret != 1 {
			if ret == -1 {
				log.Printf("Socket error: %v", err)
			} else if ret == 0 {
				log.Print("No issues found")
			} else if ret == 2 {
				log.Printf("DISCONNECTED: %v", err)
			}
			return false
		}
		diff := time.Since(start)
		p.Cl = 11
		ret2, err := d.test(p)
		if ret2 == -1 {
			log.Print(err)
			return false
		}
		if ret2 == 0 {
			ctr++
			if ctr < 3 {
				continue
			} else {
				log.Printf("Potential CLTE issue found - %s@%s://%s%s", (*p).ReqLine.Method,
					glob.URL.Scheme, glob.URL.Host, glob.URL.Path)
				GenReport(p, diff)
				return glob.ExitEarly
			}
		} else {
			log.Print("CLTE timeout on both length 5 and 6")
			return false
		}

	}
}

func GenReport(p *Payload, t time.Duration) {
	if err := createDir("/payloads/"); err != nil {
		log.Print(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		log.Print(err)
		return
	}
	fname := fmt.Sprintf("%s/payloads/%s_%s", pwd, strings.ReplaceAll(glob.URL.Host, ".", "_"), p.ReqLine.Query)
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Print(err)
		return
	}
	defer file.Close()

	for k, v := range p.Header {
		kl := strings.ToLower(strings.TrimSpace(k))
		if kl == "content-length" || kl == "transfer-encoding" {
			toFind := ""
			for i := 0; i <= 0xF; i++ {
				toFind += string(byte(i))
			}
			if !strings.ContainsAny(k, toFind) {
				break
			}
			fin := ""
			for i := 0; i < len(k); i++ {
				if k[i] <= 0xF {
					fin += fmt.Sprintf("\\x0%x", k[i])
				} else {
					fin += string(k[i])
				}
			}
			nk := fin
			fin = ""
			for i := 0; i < len(v); i++ {
				if v[i] < 0xF {
					fin += fmt.Sprintf("\\x0%x", v[i])
				} else {
					fin += string(v[i])
				}
			}
			v = fin
			p.Header[nk] = v
			delete(p.Header, k)
		}
	}
	file.WriteString("\nPoC Payload\n------------------------------\n" + (*p).ToString())
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
