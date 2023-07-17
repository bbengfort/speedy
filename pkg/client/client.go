package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/bbengfort/speedy/pkg/config"
	"golang.org/x/net/http2"
)

type Client struct {
	conf   config.ClientConfig
	client *http.Client
}

func New(conf config.ClientConfig) (_ *Client, err error) {
	var certs tls.Certificate
	if certs, err = conf.TLS.LoadCerts(); err != nil {
		return nil, err
	}

	return &Client{
		conf: conf,
		client: &http.Client{
			Transport: &http2.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:       []tls.Certificate{certs},
					InsecureSkipVerify: true,
				},
			},
		},
	}, nil
}

func (c *Client) Post(data []byte) (err error) {
	out := ioutil.NopCloser(bytes.NewReader(data))

	var req *http.Request
	if req, err = http.NewRequest(http.MethodPost, c.conf.Endpoint, out); err != nil {
		return err
	}

	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return err
	}

	defer rep.Body.Close()
	r := bufio.NewReader(rep.Body)
	buf := make([]byte, 4*1024)

	var total int
	for {
		var n int
		if n, err = r.Read(buf); err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if n > 0 {
			total += n
			fmt.Printf("%d bytes received\n", n)
		}
	}

	fmt.Printf("total sent: %d", len(data))
	fmt.Printf("total recv: %d", total)
	return nil
}
