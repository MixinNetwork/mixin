package common

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"github.com/vmihailenco/msgpack/v4"
)

//go:embed data/zstd.dic
var ZstdEmbed []byte

func init() {
	msgpack.RegisterExt(0, (*Integer)(nil))

	zstdEncoder = NewZstdEncoder(2)
	zstdDecoder = NewZstdDecoder(2)
}

var (
	// zstd --train /tmp/zstd/* -o config/data/zstd.dic
	zstdEncoder *zstd.Encoder
	zstdDecoder *zstd.Decoder

	CompressionVersionZero   = []byte{0, 0, 0, 0}
	CompressionVersionLatest = CompressionVersionZero
)

func compress(b []byte) []byte {
	b = zstdEncoder.EncodeAll(b, nil)
	return append(CompressionVersionLatest, b...)
}

func decompress(b []byte) []byte {
	header := len(CompressionVersionLatest)
	if len(b) < header*2 {
		return nil
	}

	if !bytes.Equal(b[:header], CompressionVersionZero) {
		return nil
	}
	b, err := zstdDecoder.DecodeAll(b[header:], nil)
	if err != nil {
		return nil
	}
	return b
}

func compressMsgpackMarshalPanic(val interface{}) []byte {
	payload := msgpackMarshalPanic(val)
	payload = zstdEncoder.EncodeAll(payload, nil)
	return append(CompressionVersionLatest, payload...)
}

func decompressMsgpackUnmarshal(data []byte, val interface{}) error {
	header := len(CompressionVersionLatest)
	if len(data) < header*2 {
		return msgpackUnmarshal(data, val)
	}

	version := data[:header]
	if bytes.Equal(version, CompressionVersionZero) {
		payload, err := zstdDecoder.DecodeAll(data[header:], nil)
		if err != nil {
			return err
		}
		return msgpackUnmarshal(payload, val)
	}
	return msgpackUnmarshal(data, val)
}

func msgpackMarshalPanic(val interface{}) []byte {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf).UseCompactEncoding(true).SortMapKeys(true)
	err := enc.Encode(val)
	if err != nil {
		panic(fmt.Errorf("MsgpackMarshalPanic: %#v %s", val, err.Error()))
	}
	return buf.Bytes()
}

func msgpackUnmarshal(data []byte, val interface{}) error {
	err := msgpack.Unmarshal(data, val)
	if err == nil {
		return err
	}
	return fmt.Errorf("MsgpackUnmarshal: %s %s", hex.EncodeToString(data), err.Error())
}
