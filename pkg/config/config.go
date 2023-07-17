package config

import (
	"crypto/tls"

	"github.com/kelseyhightower/envconfig"
)

const prefix = "speedy"

type ClientConfig struct {
	Endpoint string `default:"https://localhost:8765/"`
	TLS      TLSConfig
}

type ServerConfig struct {
	BindAddr string `split_words:"true" default:":8765"`
	TLS      TLSConfig
}

type TLSConfig struct {
	CertPath string `split_words:"true" required:"true"`
	KeyPath  string `split_words:"true" required:"true"`
}

func Client() (conf ClientConfig, err error) {
	if err = envconfig.Process(prefix, &conf); err != nil {
		return conf, err
	}

	return conf, nil
}

func Server() (conf ServerConfig, err error) {
	if err = envconfig.Process(prefix, &conf); err != nil {
		return conf, err
	}

	return conf, nil
}

func (t *TLSConfig) LoadCerts() (tls.Certificate, error) {
	return tls.LoadX509KeyPair(t.CertPath, t.KeyPath)
}
