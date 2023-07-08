package greatriverenergy

import (
	"net/http"
	"net/http/cookiejar"
)

type Client struct {
	client http.Client
}

func NewClient(transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	client := http.Client{
		Transport:     transport,
		CheckRedirect: nil,
		Jar:           jar,
	}
	return &Client{client: client}
}
