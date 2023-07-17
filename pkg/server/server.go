package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	speedy "github.com/bbengfort/speedy/pkg"
	"github.com/bbengfort/speedy/pkg/config"
	"github.com/hashicorp/go-multierror"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// Initializes zerolog with our default logging requirements
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "ts"
	zerolog.MessageFieldName = "msg"

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.NewConsoleWriter())
}

type Server struct {
	conf    config.ServerConfig
	mux     *httprouter.Router
	srv     *http.Server
	msgs    chan string
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
		msgs: make(chan string, 8),
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

	// NOTE: with http2 you could pub and subscribe on the same channel
	// TODO: implement a same channel pub and subscribe method
	srv.mux.POST("/", h2only(srv.publish))
	srv.mux.GET("/", h2only(srv.subscribe))
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
	log.Info().Str("listen", s.url.String()).Str("version", speedy.Version()).Msg("speedy server started")
	return <-s.errc
}

func (s *Server) Shutdown(ctx context.Context) (err error) {
	log.Info().Msg("speedy server shutting down")

	close(s.msgs)
	s.srv.SetKeepAlivesEnabled(false)
	if serr := s.srv.Shutdown(ctx); serr != nil {
		err = multierror.Append(err, serr)
	}

	return err
}

func (s *Server) publish(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Info().Msg("publisher connected")

	var nrecv uint64
	scanner := bufio.NewScanner(req.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			if err == io.EOF {
				w.Header().Set("Status", "200 OK")
				req.Body.Close()
			}
			break
		}

		s.msgs <- scanner.Text()
		nrecv++
		log.Debug().Msg("message published")
	}
	log.Info().Uint64("nrecv", nrecv).Msg("publisher disconnected")
}

func (s *Server) subscribe(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Info().Msg("subscriber connected")
	var bytes uint64

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for msg := range s.msgs {
		n, err := w.Write([]byte(msg))
		bytes += uint64(n)

		if err != nil {
			if err == io.EOF {
				w.Header().Set("Status", "200 OK")
				req.Body.Close()
			}
			break
		}
		log.Debug().Msg("message consumed")
	}
	log.Info().Uint64("bytes_written", bytes).Msg("subscriber disconnected")
}

func h2only(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// We only accept HTTP/2!
		// (Normally it's quite common to accept HTTP/1.- and HTTP/2 together.)
		if r.ProtoMajor != 2 {
			log.Warn().Str("proto", r.Proto).Msg("Not a HTTP/2 request, rejected!")
			http.Error(w, http.StatusText(http.StatusHTTPVersionNotSupported), http.StatusHTTPVersionNotSupported)
			return
		}

		// Pass through the request
		h(w, r, p)
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
