package common

import (
	"bytes"

	"github.com/vmihailenco/msgpack"
)

func init() {
	msgpack.RegisterExt(0, (*Integer)(nil))
}

var (
	CompressionVersionZero   = []byte{0, 0, 0, 0}
	CompressionVersionLatest = CompressionVersionZero
)

func CompressMsgpackMarshalPanic(val interface{}) []byte {
	payload := MsgpackMarshalPanic(val)
	return append(CompressionVersionLatest, payload...)
}

func DecompressMsgpackUnmarshal(data []byte, val interface{}) error {
	if len(data) < len(CompressionVersionLatest)*2 {
		return MsgpackUnmarshal(data, val)
	}

	version := data[:len(CompressionVersionLatest)]
	payload := data[len(CompressionVersionLatest):]
	if bytes.Compare(version, CompressionVersionZero) == 0 {
		return MsgpackUnmarshal(payload, val)
	}
	return MsgpackUnmarshal(data, val)
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
