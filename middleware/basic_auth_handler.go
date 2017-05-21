package middleware

import (
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	cost = 12
)

func hasher(s string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), cost)
	if err != nil {
		log.Fatal(errors.Wrap(err, "basic auth handler hashing error"))
	}

	return hash
}

var userHash = hasher(os.Getenv("LIBERTY_USER"))
var passHash = hasher(os.Getenv("LIBERTY_PASS"))

func BasicAuthHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || bcrypt.CompareHashAndPassword(userHash, []byte(user)) != nil || bcrypt.CompareHashAndPassword(passHash, []byte(pass)) != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm=Username and Password`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
