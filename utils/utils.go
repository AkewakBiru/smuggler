package utils

import (
	"fmt"
	"smuggler/smuggler/h2"
	"strings"
	"unicode"
)

func HexEscapeNonPrintable(s string) string {
	var res string

	for _, c := range s {
		if unicode.IsPrint(c) {
			res += string(c)
		} else {
			res = fmt.Sprintf("%s\\x%02X", res, c)
		}
	}
	return res
}

func GetH2RequestSummary(req *h2.Request) string {
	var sb strings.Builder

	sb.WriteString(HexEscapeNonPrintable(req.Method) + " " + HexEscapeNonPrintable(req.URL.EscapedPath()))
	if len(req.URL.RawQuery) > 0 {
		sb.WriteString("?" + req.URL.RawQuery)
	}
	sb.WriteString(" HTTP/2\r\n")

	for k, vv := range req.Hdrs {
		for _, v := range vv {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", HexEscapeNonPrintable(k), HexEscapeNonPrintable(v)))
		}
	}

	sb.WriteString("Host: " + HexEscapeNonPrintable(req.URL.Host) + "\r\n")
	if req.Payload != nil {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", HexEscapeNonPrintable(req.Payload.Key),
			HexEscapeNonPrintable(req.Payload.Val)))
	}

	sb.WriteString("\r\n")
	if len(req.Body) > 0 {
		sb.WriteString(string(req.Body))
	}

	return sb.String()
}
