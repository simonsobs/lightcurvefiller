package lightcurvefiller

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
)

// Creation of the HTTP Client
var HTTP_CLIENT_SET bool = false
var HTTP_CLIENT *http.Client

type HeaderTransport struct {
	Base            http.RoundTripper
	TLSClientConfig *tls.Config
	Headers         map[string]string
}

func (t *HeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to avoid mutating shared state
	req = req.Clone(req.Context())

	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	return t.base().RoundTrip(req)
}

func (t *HeaderTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	if t.TLSClientConfig != nil {
		return &http.Transport{TLSClientConfig: t.TLSClientConfig}
	}
	return http.DefaultTransport
}

func (c LightServeConfiguration) GetClient() *http.Client {
	if HTTP_CLIENT_SET {
		return HTTP_CLIENT
	}

	transport := HeaderTransport{
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	if c.use_bearer {
		transport.Headers["Authorization"] = fmt.Sprintf("Bearer %s", c.bearer)
		log.Printf("Added bearer authorization header")
	}

	if c.allow_self_signed {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		log.Printf("Added TLSClientConfig required for self-signed certificates")
	}

	HTTP_CLIENT = &http.Client{Transport: &transport}
	HTTP_CLIENT_SET = true

	return c.GetClient()
}
