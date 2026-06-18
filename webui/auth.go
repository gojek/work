package webui

import (
	"crypto/subtle"
	"net/http"
)

// AuthOptions configures optional HTTP Basic Auth for the entire Web UI. When Password is
// empty, no auth is applied and every route is served openly (the historical behavior).
// When Password is set, every route — pages, JSON API, and mutating endpoints — requires
// the credentials.
type AuthOptions struct {
	// Username is the expected Basic Auth username. Defaults to "admin" when empty.
	Username string
	// Password is the expected Basic Auth password. Empty disables auth entirely.
	Password string
	// Realm is the value placed in the WWW-Authenticate header on a 401. Defaults to "gojek/work".
	Realm string
}

// Enabled reports whether Basic Auth should wrap the Web UI.
func (o AuthOptions) Enabled() bool {
	return o.Password != ""
}

func (o AuthOptions) username() string {
	if o.Username == "" {
		return "admin"
	}
	return o.Username
}

func (o AuthOptions) realm() string {
	if o.Realm == "" {
		return "gojek/work"
	}
	return o.Realm
}

// requireBasicAuth wraps next with HTTP Basic Auth using o's credentials. On missing or
// wrong credentials it returns 401 with a WWW-Authenticate header so browsers and scripts
// (curl, etc.) know how to authenticate.
func requireBasicAuth(o AuthOptions, next http.Handler) http.Handler {
	expectedUser := []byte(o.username())
	expectedPass := []byte(o.Password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(u), expectedUser) == 1
		passOK := subtle.ConstantTimeCompare([]byte(p), expectedPass) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+o.realm()+`", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
