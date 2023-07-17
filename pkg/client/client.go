package client

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
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

func (c *Client) Publish(r io.Reader) (err error) {
	var req *http.Request
	if req, err = http.NewRequest(http.MethodPost, c.conf.Endpoint, r); err != nil {
		return err
	}

	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return err
	}

	defer rep.Body.Close()

	var data []byte
	if data, err = io.ReadAll(rep.Body); err != nil && err != io.EOF {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func (c *Client) Subscribe(w io.Writer) (err error) {
	var req *http.Request
	if req, err = http.NewRequest(http.MethodGet, c.conf.Endpoint, nil); err != nil {
		return err
	}

	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return err
	}

	defer rep.Body.Close()

	scanner := bufio.NewScanner(rep.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line := scanner.Text()
		fmt.Println(line)

		if _, err = fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
