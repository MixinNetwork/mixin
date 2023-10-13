package network

import (
	"context"
	"net"
)

const (
	TransportMessageVersion    = 2
	TransportMessageMaxSize    = 32 * 1024 * 1024
	TransportMessageHeaderSize = 6
)

type TransportMessage struct {
	Version uint8
	Size    uint32
	Data    []byte
}

type Client interface {
	RemoteAddr() net.Addr
	Receive() (*TransportMessage, error)
	Send([]byte) error
	Close() error
}

type Transport interface {
	Listen() error
	Dial(ctx context.Context) (Client, error)
	Accept(ctx context.Context) (Client, error)
	Close() error
}
