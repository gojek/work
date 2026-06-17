package webui

import (
	"crypto/subtle"
	"net/http"
)

// AdminOptions configures HTTP Basic Auth for the admin (mutating) endpoints. Admin endpoints
// are only registered when Password is non-empty, so leaving it zero-valued keeps the web UI
// fully read-only — exactly the pre-admin behavior.
type AdminOptions struct {
	// Username is the expected Basic Auth username. Defaults to "admin" when empty.
	Username string
	// Password is the expected Basic Auth password. When empty, admin endpoints are not registered.
	Password string
	// Realm is the value placed in the WWW-Authenticate header on a 401 response.
	// Defaults to "work-admin".
	Realm string
}

// Enabled reports whether admin endpoints should be registered.
func (o AdminOptions) Enabled() bool {
	return o.Password != ""
}

func (o AdminOptions) username() string {
	if o.Username == "" {
		return "admin"
	}
	return o.Username
}

func (o AdminOptions) realm() string {
	if o.Realm == "" {
		return "work-admin"
	}
	return o.Realm
}

// requireBasicAuth wraps next with HTTP Basic Auth using o's credentials. On missing or
// wrong credentials it returns 401 with a WWW-Authenticate header so browsers (and curl)
// know how to authenticate.
func requireBasicAuth(o AdminOptions, next http.Handler) http.Handler {
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
