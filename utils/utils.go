package utils

import (
	"fmt"
	"maps"
	"net/url"
	"reflect"
	"smuggler/smuggler/h2"
	"strings"
	"unicode"
)

func CloneMap(src map[string][]string) map[string][]string {
	dst := make(map[string][]string)

	for k, vv := range src {
		dst[k] = append(dst[k], vv...)
	}
	return dst
}

func ValueExists[T comparable](whole []T, val T) bool {
	for _, v := range whole {
		if v == val {
			return true
		}
	}
	return false
}

func MapValueExists[K comparable, V comparable](src map[K]any, val any) bool {
	for vv := range maps.Values(src) {
		switch t := vv.(type) {
		case V:
			if reflect.DeepEqual(t, val) {
				return true
			}
		case []V:
			for _, v := range t {
				if reflect.DeepEqual(v, val) {
					return true
				}
			}
		default:
			return false
		}
	}
	return false
}

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

func AppendQueryParam(u *url.URL, val string) {
	if len(u.Query()) > 0 {
		u.RawQuery += "&" + val
	} else {
		u.RawQuery += val
	}
}
