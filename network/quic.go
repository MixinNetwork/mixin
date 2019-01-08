package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
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

func NewQuicServer(addr string, k crypto.Key) (*QuicTransport, error) {
	tlsConf, err := generateTLSConfig(k)
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
	m.Data = make([]byte, m.Size)
	s, err = c.stream.Read(m.Data)
	if err != nil {
		return nil, err
	}
	if s != int(m.Size) {
		return nil, fmt.Errorf("quic receive invalid message data %d", s)
	}
	return m.Data, nil
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
	}
	err := c.stream.SetWriteDeadline(time.Now().Add(WriteDeadline))
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

func generateTLSConfig(k crypto.Key) (*tls.Config, error) {
	var priv *ecdsa.PrivateKey
	key := new(big.Int)
	key.SetBytes(k[:])
	priv = new(ecdsa.PrivateKey)
	curve := elliptic.P256()
	priv.PublicKey.Curve = curve
	priv.D = key
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(key.Bytes())

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}, nil
}
