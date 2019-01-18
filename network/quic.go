package network

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const (
	MaxIncomingStreams = 64
	HandshakeTimeout   = 2 * time.Second
	IdleTimeout        = 3 * time.Second
	ReadDeadline       = 3 * time.Second
	WriteDeadline      = 3 * time.Second
)

type QuicClient struct {
	session quic.Session
	stream  quic.Stream
}

type QuicTransport struct {
	addr     string
	tls      *tls.Config
	listener quic.Listener
}

func NewQuicServer(addr string) (*QuicTransport, error) {
	tlsConf, err := generateTLSConfig()
	if err != nil {
		return nil, err
	}

	return &QuicTransport{
		addr: addr,
		tls:  tlsConf,
	}, nil
}

func NewQuicClient(addr string) (*QuicTransport, error) {
	return &QuicTransport{
		addr: addr,
		tls:  &tls.Config{InsecureSkipVerify: true},
	}, nil
}

func (t *QuicTransport) Dial() (Client, error) {
	sess, err := quic.DialAddr(t.addr, t.tls, &quic.Config{
		MaxIncomingStreams: MaxIncomingStreams,
		HandshakeTimeout:   HandshakeTimeout,
		IdleTimeout:        IdleTimeout,
		KeepAlive:          false,
	})
	if err != nil {
		return nil, err
	}
	stm, err := sess.OpenStreamSync()
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session: sess,
		stream:  stm,
	}, nil
}

func (t *QuicTransport) Listen() error {
	l, err := quic.ListenAddr(t.addr, t.tls, &quic.Config{
		MaxIncomingStreams: MaxIncomingStreams,
		HandshakeTimeout:   HandshakeTimeout,
		IdleTimeout:        IdleTimeout,
		KeepAlive:          false,
	})
	if err != nil {
		return err
	}
	t.listener = l
	return nil
}

func (t *QuicTransport) Accept() (Client, error) {
	sess, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}
	stm, err := sess.AcceptStream()
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session: sess,
		stream:  stm,
	}, nil
}

func (c *QuicClient) Receive() ([]byte, error) {
	err := c.stream.SetReadDeadline(time.Now().Add(ReadDeadline))
	if err != nil {
		return nil, err
	}
	var m TransportMessage
	header := make([]byte, TransportMessageHeaderSize)
	s, err := c.stream.Read(header)
	if err != nil {
		return nil, err
	}
	if s != TransportMessageHeaderSize {
		return nil, fmt.Errorf("quic receive invalid message header size %d", s)
	}
	m.Version = header[0]
	if m.Version != TransportMessageVersion {
		return nil, fmt.Errorf("quic receive invalid message version %d", m.Version)
	}
	m.Size = binary.BigEndian.Uint32(header[1:])
	if m.Size > TransportMessageMaxSize {
		return nil, fmt.Errorf("quic receive invalid message size %d", m.Size)
	}
	data := make([]byte, m.Size)
	s, err = c.stream.Read(data)
	if err != nil {
		return nil, err
	}
	if s != int(m.Size) {
		return nil, fmt.Errorf("quic receive invalid message data %d", s)
	}

	gzReader, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	m.Data, err = ioutil.ReadAll(gzReader)
	return m.Data, err
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
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

	err = c.stream.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))
	_, err = c.stream.Write(header)
	if err != nil {
		return err
	}
	_, err = c.stream.Write(data)
	return err
}

func (c *QuicClient) Close() error {
	c.stream.Close()
	return c.session.Close()
}

func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}, nil
}
