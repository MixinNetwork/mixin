package network

const (
	MessageVersion    = 1
	MessageMaxSize    = 32 * 1024 * 1024
	MessageHeaderSize = 5
)

type Message struct {
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
