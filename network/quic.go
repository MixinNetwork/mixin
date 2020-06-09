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
	"net"
	"time"

	"github.com/gobuffalo/packr"
	"github.com/lucas-clemente/quic-go"
	"github.com/valyala/gozstd"
)

const (
	MaxIncomingStreams = 128
	HandshakeTimeout   = 10 * time.Second
	IdleTimeout        = 60 * time.Second
	ReadDeadline       = 10 * time.Second
	WriteDeadline      = 10 * time.Second
)

type QuicClient struct {
	session      quic.Session
	send         quic.SendStream
	receive      quic.ReceiveStream
	zstdZipper   *gozstd.CDict
	zstdUnzipper *gozstd.DDict
	gzipZipper   *gzip.Writer
	gzipUnzipper *gzip.Reader
}

type QuicTransport struct {
	addr     string
	tls      *tls.Config
	listener quic.Listener
}

func NewQuicServer(addr string) (*QuicTransport, error) {
	tlsConf := generateTLSConfig()
	return &QuicTransport{
		addr: addr,
		tls:  tlsConf,
	}, nil
}

func NewQuicClient(addr string) (*QuicTransport, error) {
	return &QuicTransport{
		addr: addr,
		tls: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"mixin-quic-peer"},
		},
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
	box := packr.NewBox("../config/data")
	dic, err := box.Find("zstd.dic")
	if err != nil {
		return nil, err
	}
	cdict, err := gozstd.NewCDictLevel(dic, 5)
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session:    sess,
		send:       stm,
		zstdZipper: cdict,
		gzipZipper: zipper,
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

func (t *QuicTransport) Close() error {
	return t.listener.Close()
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
	box := packr.NewBox("../config/data")
	dic, err := box.Find("zstd.dic")
	if err != nil {
		return nil, err
	}
	ddict, err := gozstd.NewDDict(dic)
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session:      sess,
		receive:      stm,
		zstdUnzipper: ddict,
		gzipUnzipper: new(gzip.Reader),
	}, nil
}

func (c *QuicClient) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
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
	m.Compression = header[1]
	if m.Compression != TransportCompressionGzip && m.Compression != TransportCompressionZstd {
		return nil, fmt.Errorf("quic receive invalid message compression %d", m.Compression)
	}
	m.Size = binary.BigEndian.Uint32(header[2:])
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

	switch m.Compression {
	case TransportCompressionGzip:
		err = c.gzipUnzipper.Reset(bytes.NewBuffer(m.Data))
		if err != nil {
			return nil, err
		}
		defer c.gzipUnzipper.Close()
		m.Data, err = ioutil.ReadAll(c.gzipUnzipper)
	case TransportCompressionZstd:
		m.Data, err = gozstd.DecompressDict(nil, m.Data, c.zstdUnzipper)
	}

	return m.Data, err
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
	}

	switch TransportCompressionMethod {
	case TransportCompressionGzip:
		var buf bytes.Buffer
		c.gzipZipper.Reset(&buf)
		_, err := c.gzipZipper.Write(data)
		if err != nil {
			return err
		}
		err = c.gzipZipper.Close()
		if err != nil {
			return err
		}
		data = buf.Bytes()
	case TransportCompressionZstd:
		data = gozstd.CompressDict(nil, data, c.zstdZipper)
	}

	err := c.send.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, TransportCompressionMethod, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[2:], uint32(len(data)))
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

func generateTLSConfig() *tls.Config {
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
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"mixin-quic-peer"},
	}
}
