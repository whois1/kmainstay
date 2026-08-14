package httpapi_test

import (
	"net/http"
	"net/http/cookiejar"
)

func cookieJar() (http.CookieJar, error) { return cookiejar.New(nil) }
