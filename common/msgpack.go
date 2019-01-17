package common

import (
	"bytes"

	"github.com/vmihailenco/msgpack"
)

func MsgpackMarshalPanic(val interface{}) []byte {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf).UseCompactEncoding(true)
	err := enc.Encode(val)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}
