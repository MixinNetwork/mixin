package network

const (
	TransportMessageVersion    = 1
	TransportMessageMaxSize    = 32 * 1024 * 1024
	TransportMessageHeaderSize = 5
)

type TransportMessage struct {
	Version uint8
	Size    uint32
	Data    []byte
}

type Client interface {
	Receive() ([]byte, error)
	Send([]byte) error
	Close() error
}

type Transport interface {
	Listen() error
	Dial() (Client, error)
	Accept() (Client, error)
}
