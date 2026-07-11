package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/stretchr/testify/require"
)

func TestRPCRejectsOversizedAndTrailingRequestBodies(t *testing.T) {
	impl := &RPC{custom: &config.Custom{}}

	for name, body := range map[string]string{
		"trailing json": `{"method":"unknown","params":[]}{}`,
		"oversized":     `{"method":"unknown","params":[]}` + strings.Repeat(" ", maxRPCRequestBodySize),
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(body))
			res := httptest.NewRecorder()
			impl.ServeHTTP(res, req)
			require.Contains(t, res.Body.String(), "bad request")
		})
	}
}

func TestRPCServerSetsHeaderTimeout(t *testing.T) {
	server := NewServer(&config.Custom{}, nil, nil, 6860)
	require.Equal(t, 5*time.Second, server.ReadHeaderTimeout)
}
