package network

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"time"
)

type TcpClient struct {
	conn net.Conn
}

type TcpTransport struct {
	addr     string
	listener net.Listener
}

func NewTcpServer(addr string) (*TcpTransport, error) {
	return &TcpTransport{
		addr: addr,
	}, nil
}

func NewTcpClient(addr string) (*TcpTransport, error) {
	return &TcpTransport{
		addr: addr,
	}, nil
}

func (t *TcpTransport) Dial() (Client, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return &TcpClient{
		conn: conn,
	}, nil
}

func (t *TcpTransport) Listen() error {
	l, err := net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}
	t.listener = l
	return nil
}

func (t *TcpTransport) Accept() (Client, error) {
	conn, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}
	return &TcpClient{
		conn: conn,
	}, nil
}

func (c *TcpClient) Receive() ([]byte, error) {
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
		return nil, fmt.Errorf("tcp receive invalid message header size %d", s)
	}
	m.Version = header[0]
	if m.Version != TransportMessageVersion {
		return nil, fmt.Errorf("tcp receive invalid message version %d", m.Version)
	}
	m.Size = binary.BigEndian.Uint32(header[1:])
	if m.Size > TransportMessageMaxSize {
		return nil, fmt.Errorf("tcp receive invalid message size %d", m.Size)
	}
	data := make([]byte, m.Size)
	s, err = c.conn.Read(data)
	if err != nil {
		return nil, err
	}
	if s != int(m.Size) {
		return nil, fmt.Errorf("tcp receive invalid message data %d %d", s, m.Size)
	}

	gzReader, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	m.Data, err = ioutil.ReadAll(gzReader)
	return m.Data, err
}

func (c *TcpClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("tcp send invalid message size %d", l)
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

func (c *TcpClient) Close() error {
	return c.conn.Close()
}
