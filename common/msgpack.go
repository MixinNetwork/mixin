package common

import (
	"bytes"

	"github.com/gobuffalo/packr"
	"github.com/valyala/gozstd"
	"github.com/vmihailenco/msgpack"
)

func init() {
	msgpack.RegisterExt(0, (*Integer)(nil))

	box := packr.NewBox("../config/data")
	dic, err := box.Find("zstd.dic")
	if err != nil {
		panic(err)
	}
	zstdCDict, err = gozstd.NewCDictLevel(dic, 5)
	if err != nil {
		panic(err)
	}
	zstdDDict, err = gozstd.NewDDict(dic)
	if err != nil {
		panic(err)
	}
}

var (
	zstdCDict *gozstd.CDict
	zstdDDict *gozstd.DDict

	CompressionVersionZero   = []byte{0, 0, 0, 0}
	CompressionVersionLatest = CompressionVersionZero
)

func CompressMsgpackMarshalPanic(val interface{}) []byte {
	payload := MsgpackMarshalPanic(val)
	payload = gozstd.CompressDict(nil, payload, zstdCDict)
	return append(CompressionVersionLatest, payload...)
}

func DecompressMsgpackUnmarshal(data []byte, val interface{}) error {
	header := len(CompressionVersionLatest)
	if len(data) < header*2 {
		return MsgpackUnmarshal(data, val)
	}

	version := data[:header]
	if bytes.Compare(version, CompressionVersionZero) == 0 {
		payload, err := gozstd.DecompressDict(nil, data[header:], zstdDDict)
		if err != nil {
			return err
		}
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
