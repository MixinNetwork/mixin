package network

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

type UnixClient struct {
	conn net.Conn
}

type UnixTransport struct {
	addr     string
	listener net.Listener
}

func NewUnixServer(addr string) (*UnixTransport, error) {
	addr = "/tmp/" + hex.EncodeToString([]byte(addr))
	return &UnixTransport{
		addr: addr,
	}, nil
}

func NewUnixClient(addr string) (*UnixTransport, error) {
	addr = addr[strings.Index(addr, ":"):]
	addr = "/tmp/" + hex.EncodeToString([]byte(addr))
	return &UnixTransport{
		addr: addr,
	}, nil
}

func (t *UnixTransport) Dial() (Client, error) {
	conn, err := net.Dial("unix", t.addr)
	if err != nil {
		return nil, err
	}
	return &UnixClient{
		conn: conn,
	}, nil
}

func (t *UnixTransport) Listen() error {
	l, err := net.Listen("unix", t.addr)
	if err != nil {
		return err
	}
	t.listener = l
	return nil
}

func (t *UnixTransport) Accept() (Client, error) {
	conn, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}
	return &UnixClient{
		conn: conn,
	}, nil
}

func (c *UnixClient) Receive() ([]byte, error) {
	err := c.conn.SetReadDeadline(time.Now().Add(ReadDeadline))
	if err != nil {
		return nil, err
	}
	var m TransportMessage
	header := make([]byte, TransportMessageHeaderSize)
	s, err := c.conn.Read(header)
	if err != nil {
		return nil, err
	}
	if s != TransportMessageHeaderSize {
		return nil, fmt.Errorf("unix receive invalid message header size %d", s)
	}
	m.Version = header[0]
	if m.Version != TransportMessageVersion {
		return nil, fmt.Errorf("unix receive invalid message version %d", m.Version)
	}
	m.Size = binary.BigEndian.Uint32(header[1:])
	if m.Size > TransportMessageMaxSize {
		return nil, fmt.Errorf("unix receive invalid message size %d", m.Size)
	}
	for {
		data := make([]byte, int(m.Size)-len(m.Data))
		s, err = c.conn.Read(data)
		if err != nil {
			return nil, err
		}
		m.Data = append(m.Data, data[:s]...)
		if len(m.Data) == int(m.Size) {
			break
		}
	}

	gzReader, err := gzip.NewReader(bytes.NewBuffer(m.Data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	m.Data, err = ioutil.ReadAll(gzReader)
	return m.Data, err
}

func (c *UnixClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("unix send invalid message size %d", l)
	}

	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, 3)
	if err != nil {
		return err
	}
	_, err = gzWriter.Write(data)
	if err != nil {
		return err
	}
	err = gzWriter.Close()
	if err != nil {
		return err
	}
	data = buf.Bytes()

	err = c.conn.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))
	_, err = c.conn.Write(header)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(data)
	return err
}

func (c *UnixClient) Close() error {
	return c.conn.Close()
}
