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
	MaxIncomingStreams = 128
	HandshakeTimeout   = 10 * time.Second
	IdleTimeout        = 60 * time.Second
	ReadDeadline       = 10 * time.Second
	WriteDeadline      = 10 * time.Second
)

type QuicClient struct {
	session  quic.Session
	send     quic.SendStream
	receive  quic.ReceiveStream
	zipper   *gzip.Writer
	unzipper *gzip.Reader
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
		KeepAlive:          true,
	})
	if err != nil {
		return nil, err
	}
	stm, err := sess.OpenUniStreamSync()
	if err != nil {
		return nil, err
	}
	zipper, err := gzip.NewWriterLevel(nil, 3)
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session: sess,
		send:    stm,
		zipper:  zipper,
	}, nil
}

func (t *QuicTransport) Listen() error {
	l, err := quic.ListenAddr(t.addr, t.tls, &quic.Config{
		MaxIncomingStreams: MaxIncomingStreams,
		HandshakeTimeout:   HandshakeTimeout,
		IdleTimeout:        IdleTimeout,
		KeepAlive:          true,
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
	stm, err := sess.AcceptUniStream()
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session:  sess,
		receive:  stm,
		unzipper: new(gzip.Reader),
	}, nil
}

func (c *QuicClient) Receive() ([]byte, error) {
	err := c.receive.SetReadDeadline(time.Now().Add(ReadDeadline))
	if err != nil {
		return nil, err
	}
	var m TransportMessage
	header := make([]byte, TransportMessageHeaderSize)
	s, err := c.receive.Read(header)
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
	for {
		data := make([]byte, int(m.Size)-len(m.Data))
		s, err = c.receive.Read(data)
		if err != nil {
			return nil, err
		}
		m.Data = append(m.Data, data[:s]...)
		if len(m.Data) == int(m.Size) {
			break
		}
	}

	err = c.unzipper.Reset(bytes.NewBuffer(m.Data))
	if err != nil {
		return nil, err
	}
	defer c.unzipper.Close()

	m.Data, err = ioutil.ReadAll(c.unzipper)
	return m.Data, err
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
	}

	var buf bytes.Buffer
	c.zipper.Reset(&buf)
	_, err := c.zipper.Write(data)
	if err != nil {
		return err
	}
	err = c.zipper.Close()
	if err != nil {
		return err
	}
	data = buf.Bytes()

	err = c.send.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))
	_, err = c.send.Write(header)
	if err != nil {
		return err
	}
	_, err = c.send.Write(data)
	return err
}

func (c *QuicClient) Close() error {
	if c.send != nil {
		c.send.Close()
	}
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
