package credhub_test

import (
	"net/http"
	"net/http/httptest"
)

func mockUaaServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(400)
				return
			}

			if r.FormValue("grant_type") != "client_credentials" {
				w.WriteHeader(400)
				return
			}

			var user, pass string
			var ok bool
			if user, pass, ok = r.BasicAuth(); !ok {
				w.WriteHeader(401)
				return
			}

			if user == "user" && pass == "pass" {
				w.Header().Add("content-type", "application/json")

				out := `{"access_token": "abcd"}`
				w.Write([]byte(out))
			} else {
				w.WriteHeader(401)
				return
			}
		} else {
			w.WriteHeader(404)
		}
	}))
}
