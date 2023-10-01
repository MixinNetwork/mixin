package common

import (
	"bytes"
	_ "embed"

	"github.com/klauspost/compress/zstd"
)

// FIXME update this to the latest data format
//
//go:embed data/zstd.dic
var ZstdEmbed []byte

func init() {
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
