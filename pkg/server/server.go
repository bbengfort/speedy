package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/bbengfort/speedy/pkg/config"
	"github.com/hashicorp/go-multierror"
	"github.com/julienschmidt/httprouter"
)

type Server struct {
	conf    config.ServerConfig
	mux     *httprouter.Router
	srv     *http.Server
	started time.Time
	url     *url.URL
	errc    chan error
}

func New(conf config.ServerConfig) (_ *Server, err error) {
	var certs tls.Certificate
	if certs, err = conf.TLS.LoadCerts(); err != nil {
		return nil, err
	}

	mux := httprouter.New()
	srv := &Server{
		conf: conf,
		mux:  mux,
		srv: &http.Server{
			Addr:    conf.BindAddr,
			Handler: mux,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{certs},
			},
			ErrorLog:          nil,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       5 * time.Minute,
		},
		errc: make(chan error),
	}

	srv.mux.POST("/", srv.handler)
	return srv, nil
}

func (s *Server) Serve() (err error) {
	// Catch OS signals for graceful shutdowns
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	go func() {
		<-quit

		// Require the shutdown occurs in 10 seconds without blocking
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.errc <- s.Shutdown(ctx)
	}()

	var sock net.Listener
	if sock, err = net.Listen("tcp", s.srv.Addr); err != nil {
		return fmt.Errorf("could not listen on bind addr %s: %w", s.srv.Addr, err)
	}

	s.setURL(sock.Addr())
	go func(sock net.Listener) {
		if serr := s.srv.ServeTLS(sock, s.conf.TLS.CertPath, s.conf.TLS.KeyPath); !errors.Is(serr, http.ErrServerClosed) {
			s.errc <- serr
		}
		s.errc <- nil
	}(sock)

	s.started = time.Now()
	return <-s.errc
}

func (s *Server) Shutdown(ctx context.Context) (err error) {
	s.srv.SetKeepAlivesEnabled(false)
	if serr := s.srv.Shutdown(ctx); serr != nil {
		err = multierror.Append(err, serr)
	}

	return err
}

func (s *Server) handler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// We only accept HTTP/2!
	// (Normally it's quite common to accept HTTP/1.- and HTTP/2 together.)
	if req.ProtoMajor != 2 {
		log.Println("Not a HTTP/2 request, rejected!")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 4*1024)

	for {
		n, err := req.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}

		if err != nil {
			if err == io.EOF {
				w.Header().Set("Status", "200 OK")
				req.Body.Close()
			}
			break
		}
	}
}

func (s *Server) setURL(addr net.Addr) {
	s.url = &url.URL{
		Scheme: "https",
		Host:   addr.String(),
	}

	if tcp, ok := addr.(*net.TCPAddr); ok && tcp.IP.IsUnspecified() {
		s.url.Host = fmt.Sprintf("127.0.0.1:%d", tcp.Port)
	}
}
