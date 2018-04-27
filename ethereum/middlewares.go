package main

import (
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"mixin.one/session"
	"mixin.one/views"
)

const publicKey = ""

func AuthenticateMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			views.RenderErrorResponse(w, r, session.AuthorizationError(r.Context()))
			return
		}
		token, err := jwt.Parse(header[7:], func(token *jwt.Token) (interface{}, error) {
			_, ok := token.Method.(*jwt.SigningMethodRSA)
			if !ok {
				return nil, session.BadDataError(r.Context())
			}
			secretBytes, err := base64.StdEncoding.DecodeString(publicKey)
			if err != nil {
				return nil, session.ServerError(r.Context(), err)
			}
			pub, err := x509.ParsePKIXPublicKey(secretBytes)
			if err != nil {
				return nil, session.ServerError(r.Context(), err)
			}
			return pub, nil
		})
		if err != nil || !token.Valid {
			views.RenderErrorResponse(w, r, session.AuthorizationError(r.Context()))
			return
		}
		handler.ServeHTTP(w, r)
	})
}
