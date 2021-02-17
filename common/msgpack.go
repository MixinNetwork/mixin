package common

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"fmt"

	"github.com/valyala/gozstd"
	"github.com/vmihailenco/msgpack/v4"
)

//go:embed data/zstd.dic
var ZstdEmbed []byte

func init() {
	msgpack.RegisterExt(0, (*Integer)(nil))

	zcd, err := gozstd.NewCDictLevel(ZstdEmbed, 5)
	if err != nil {
		panic(err)
	}
	zdd, err := gozstd.NewDDict(ZstdEmbed)
	if err != nil {
		panic(err)
	}

	zstdCDict = zcd
	zstdDDict = zdd
}

var (
	// zstd --train /tmp/zstd/* -o config/data/zstd.dic
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
	if bytes.Equal(version, CompressionVersionZero) {
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
		panic(fmt.Errorf("MsgpackMarshalPanic: %#v %s", val, err.Error()))
	}
	return buf.Bytes()
}

func MsgpackUnmarshal(data []byte, val interface{}) error {
	err := msgpack.Unmarshal(data, val)
	if err == nil {
		return err
	}
	return fmt.Errorf("MsgpackUnmarshal: %s %s", hex.EncodeToString(data), err.Error())
}
