package h1

import (
	"fmt"
	"net/url"
)

const RN = "\r\n"

type Payload struct {
	URL    url.URL
	Method string
	Header map[string]string // a key value pair of HTTP headers
	Body   string            // body of the request
	Cl     int               // content-length
	HdrPl  string            // optional header payload
}

func (p *Payload) ToString() string {
	var final string
	final = fmt.Sprintf("%s %s", p.Method, p.URL.EscapedPath())
	if len(p.URL.RawQuery) > 0 {
		final += "?" + p.URL.RawQuery
	}
	if len(p.URL.Fragment) > 0 {
		final = fmt.Sprintf("%s#%s", final, p.URL.Fragment)
	}
	final += " HTTP/1.1\r\n"
	for k, v := range p.Header {
		if len(v) == 0 {
			continue
		}
		final += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	if len(p.HdrPl) > 0 {
		final += p.HdrPl + RN
	}
	if p.Cl > 0 {
		final += fmt.Sprintf("Content-Length: %d\r\n", p.Cl)
	}
	if len(p.Body) > 0 { // it doesn't matter the type of method, if there is a body, i will just send it
		final += fmt.Sprintf("%s%s", RN, p.Body)
	} else {
		final += RN
	}
	return final
}
