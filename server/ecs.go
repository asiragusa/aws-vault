package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

func writeErrorMessage(w http.ResponseWriter, msg string, status int) {
	err := json.NewEncoder(w).Encode(map[string]string{"Message": msg})
	if err != nil {
		http.Error(w, err.Error(), status)
	}
}

func withAuthorizationCheck(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != token {
			writeErrorMessage(w, "invalid Authorization token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func StartEcsCredentialServer(creds *credentials.Credentials) (string, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", err
	}
	token, err := generateRandomString()
	if err != nil {
		return "", "", err
	}

	go func() {
		err := http.Serve(listener, withAuthorizationCheck(token, ecsCredsHandler(creds)))
		// returns ErrServerClosed on graceful close
		if err != http.ErrServerClosed {
			log.Fatalf("Serve(): %s", err)
		}
	}()

	uri := fmt.Sprintf("http://%s", listener.Addr().String())
	return uri, token, nil
}

func ecsCredsHandler(creds *credentials.Credentials) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val, err := creds.Get()
		if err != nil {
			writeErrorMessage(w, err.Error(), http.StatusInternalServerError)
			return
		}

		credsExpiresAt, err := creds.ExpiresAt()
		if err != nil {
			writeErrorMessage(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(map[string]string{
			"AccessKeyId":     val.AccessKeyID,
			"SecretAccessKey": val.SecretAccessKey,
			"Token":           val.SessionToken,
			"Expiration":      credsExpiresAt.Format("2006-01-02T15:04:05Z"),
		})
		if err != nil {
			writeErrorMessage(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func generateRandomString() (string, error) {
	b := make([]byte, 30)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}