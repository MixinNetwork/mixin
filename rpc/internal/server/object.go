package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	defaultTextPlainType = "text/plain; charset=utf-8"
	defaultJSONType      = "application/json; charset=utf-8"
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
		w.Header().Set("Content-Type", defaultTextPlainType)
	} else if m := parseJSON(tx.Extra); m == nil {
		w.Header().Set("Content-Type", decideContentType(tx.Extra))
	} else if len(ps) < 4 {
		w.Header().Set("Content-Type", defaultJSONType)
	} else {
		v, mime := parseDataURI(fmt.Sprint(m[ps[3]]))
		w.Header().Set("Content-Type", mime)
		b = v
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(b)
	if err != nil {
		panic(err)
	}
}

func parseDataURI(v string) ([]byte, string) {
	const scheme = "data:"
	if !strings.HasPrefix(v, scheme) {
		return []byte(v), defaultTextPlainType
	}
	ds := strings.Split(v, ",")
	if len(ds) != 2 {
		return []byte(v), defaultTextPlainType
	}
	ds[0] = ds[0][len(scheme):]

	ms := strings.Split(ds[0], ";")
	if len(ms) < 2 {
		return []byte(ds[1]), defaultTextPlainType
	}

	mime, data := ms[0], ds[1]
	if ms[len(ms)-1] == "base64" {
		b, _ := base64.StdEncoding.DecodeString(data)
		data = string(b)
	}
	if mime == "" || !utf8.ValidString(mime) {
		mime = decideContentType([]byte(data))
	}
	return []byte(data), mime
}

func decideContentType(extra []byte) string {
	if utf8.ValidString(string(extra)) {
		return defaultTextPlainType
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
