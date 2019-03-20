package common

import (
	"bytes"

	"github.com/MixinNetwork/msgpack"
)

func init() {
	msgpack.RegisterExt(0, (*Integer)(nil))
}

func MsgpackMarshalPanic(val interface{}) []byte {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf).UseCompactEncoding(true)
	err := enc.Encode(val)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func MsgpackUnmarshal(data []byte, val interface{}) error {
	return msgpack.Unmarshal(data, val)
}
