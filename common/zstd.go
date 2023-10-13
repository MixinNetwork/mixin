package common

import (
	"runtime"

	"github.com/klauspost/compress/zstd"
)

func NewZstdDecoder(ccr int, dict []byte) *zstd.Decoder {
	if ccr > runtime.GOMAXPROCS(0) {
		ccr = runtime.GOMAXPROCS(0)
	}
	opts := []zstd.DOption{
		zstd.WithDecoderConcurrency(ccr),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(1024 * 1024 * 16),
	}
	if dict != nil {
		opts = append(opts, zstd.WithDecoderDicts(dict))
	}
	dec, err := zstd.NewReader(nil, opts...)
	if err != nil {
		panic(err)
	}
	return dec
}

func NewZstdEncoder(ccr int, dict []byte) *zstd.Encoder {
	if ccr > runtime.GOMAXPROCS(0) {
		ccr = runtime.GOMAXPROCS(0)
	}
	opts := []zstd.EOption{
		zstd.WithEncoderConcurrency(ccr),
		zstd.WithEncoderLevel(3),
		zstd.WithWindowSize(1024 * 32),
		zstd.WithEncoderCRC(false),
	}
	if dict != nil {
		opts = append(opts, zstd.WithEncoderDict(dict))
	}
	enc, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		panic(err)
	}
	return enc
}
