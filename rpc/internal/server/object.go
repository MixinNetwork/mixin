package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (impl *RPC) handleObject(w http.ResponseWriter, r *http.Request, rdr *Render) {
	ps := strings.Split(r.URL.Path, "/")
	if len(ps) < 3 || ps[1] != "objects" {
		rdr.RenderError(fmt.Errorf("bad request %s %s", r.Method, r.URL.Path))
		return
	}
	txHash, err := crypto.HashFromString(ps[2])
	if err != nil {
		rdr.RenderError(fmt.Errorf("bad request %s %s", r.Method, r.URL.Path))
		return
	}

	tx, _, err := impl.Store.ReadTransaction(txHash)
	if err != nil {
		rdr.RenderError(err)
		return
	}
	if tx == nil || tx.Asset != common.XINAssetId {
		rdr.RenderError(fmt.Errorf("not found %s", r.URL.Path))
		return
	}

	b := tx.Extra
	w.Header().Set("Cache-Control", "max-age=31536000, public")
	if len(tx.Extra) == 0 {
		w.Header().Set("Content-Type", "text/plain")
	} else if m := parseJSON(tx.Extra); m == nil {
		w.Header().Set("Content-Type", decideContentType(tx.Extra))
	} else if len(ps) < 4 {
		w.Header().Set("Content-Type", "application/json")
	} else {
		v, mime := parseDataURI(fmt.Sprint(m[ps[3]]))
		w.Header().Set("Content-Type", mime)
		b = v
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func parseDataURI(v string) ([]byte, string) {
	ds := strings.Split(v, ",")
	if len(ds) != 2 {
		return []byte(v), "text/plain"
	}
	s := tryURLQueryUnescape(ds[1])

	ms := strings.Split(ds[0], ";")
	if len(ms) < 2 {
		return []byte(s), "text/plain"
	}
	if ms[len(ms)-1] == "base64" {
		b, _ := base64.StdEncoding.DecodeString(s)
		s = string(b)
	}
	if ms[0] == "" || !utf8.ValidString(ms[0]) {
		return []byte(s), decideContentType([]byte(s))
	}
	return []byte(s), ms[0]
}

func tryURLQueryUnescape(v string) string {
	s, err := url.QueryUnescape(v)
	if err != nil {
		return v
	}
	return s
}

func decideContentType(extra []byte) string {
	if utf8.ValidString(string(extra)) {
		return "text/plain"
	} else {
		return "application/octet-stream"
	}
}

func parseJSON(extra []byte) map[string]any {
	if extra[0] != '{' && extra[0] != '[' {
		return nil
	}
	var r map[string]any
	err := json.Unmarshal(extra, &r)
	if err != nil {
		return nil
	}
	return r
}
