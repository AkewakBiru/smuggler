package smuggler

import (
	"fmt"
	"net/http"
)

const RN = "\r\n"
const ENDCHUNK = "0\r\n\r\n"

type ReqLine struct {
	Method   string
	Path     string // endpoint (directory/ page) the request is sent to
	Fragment string
	Query    string // query parameter and value combn
	Version  string
}

type Payload struct {
	ReqLine ReqLine
	Header  map[string]string // a key value pair of HTTP headers
	Body    string            // body of the request
	Host    string            // host the request is sent to
	Port    int               // destination port of the request
	Cl      int               // content-length
}

func (p *Payload) ToString() string {
	var final string
	final = fmt.Sprintf("%s %s", p.ReqLine.Method, p.ReqLine.Path)
	if len(p.ReqLine.Query) > 0 {
		final += "?" + p.ReqLine.Query
	}
	if len(p.ReqLine.Fragment) > 0 {
		final += "#" + p.ReqLine.Fragment
	}
	final += fmt.Sprintf(" %s%s", p.ReqLine.Version, RN)
	for k, v := range p.Header {
		final += fmt.Sprintf("%s: %s%s", k, v, RN)
	}
	if p.Cl > 0 {
		final += fmt.Sprintf("Content-Length: %d%s", p.Cl, RN)
	}

	if p.ReqLine.Method == http.MethodGet || p.ReqLine.Method == http.MethodHead || p.ReqLine.Method == http.MethodOptions {
		final += RN
	} else if p.ReqLine.Method == "POST" {
		if len(p.Body) > 0 {
			final += fmt.Sprintf("%s%s", RN, p.Body)
		} else {
			final += RN
		}
	}
	return final
}
