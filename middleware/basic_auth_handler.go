package middleware

import (
	"crypto/subtle"
	"fmt"
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

func BasicAuthHandler(handler http.Handler) http.Handler {
	userHash := hasher(os.Getenv("LIBERTY_USER"))
	passHash := hasher(os.Getenv("LIBERTY_PASS"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		fmt.Println("BASIC_AUTH", user, pass)
		fmt.Println("BASIC_HASHED", string(userHash), string(passHash))
		fmt.Println("CREDS_HASHED", string(hasher(user)), string(hasher(pass)))
		if !ok || subtle.ConstantTimeCompare(hasher(user),
			userHash) != 1 || subtle.ConstantTimeCompare(hasher(pass), passHash) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm=Username and Password`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
