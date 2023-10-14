package crypto

import (
	"crypto/rand"
	"fmt"
)

type randReader struct{}

func RandReader() *randReader {
	return &randReader{}
}

func (r *randReader) Read(b []byte) (n int, err error) {
	ReadRand(b)
	return len(b), nil
}

func ReadRand(buf []byte) {
	if len(buf) == 0 {
		panic(buf)
	}
	n, err := rand.Read(buf)
	if err != nil || len(buf) != n {
		panic(err)
	}
	if len(buf) < 4 {
		return
	}
	set := make(map[byte]int)
	for _, b := range buf {
		set[b] += 1
	}
	for k, v := range set {
		if v < len(buf)/3 {
			continue
		}
		panic(fmt.Errorf("entropy not enough %d %d", k, v))
	}
}
