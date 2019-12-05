package server

import (
	"os"

	"github.com/spf13/cobra"
)

type Registry interface {
	Register(interface{}, ...interface{}) (func(...interface{}) (interface{}, error), error)
}

type ServerConfig struct {
	Port int
	TLS  struct {
		Port int
		Cert string
		Key  string
	}

	MaxConns                   int64
	MaxConcurrentTLSHandshakes int64

	Handler func() Handler
}

type Service struct {
	Server *Server
	config *ServerConfig
}

func RegisterService(r Registry) {
	r.Register(func(cmd *cobra.Command, config *ServerConfig) (*Service, error) {
		s := &Service{
			config: config,
		}
		s.configureFlags(cmd)

		return s, nil
	})
}

func (s *Service) configureFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.IntVar(
		&s.config.Port,
		"server-port", s.config.Port,
		"HTTP server port",
	)

	flags.IntVar(
		&s.config.TLS.Port,
		"server-tls-port", 0,
		"HTTPS server port",
	)

	flags.StringVar(
		&s.config.TLS.Cert,
		"server-tls-cert", "",
		"HTTPS server certificate",
	)

	flags.StringVar(
		&s.config.TLS.Key,
		"server-tls-key", "",
		"HTTPS server certificate key",
	)

}

func (s *Service) Init() error {
	var err error

	s.Server, err = NewServerWithTLS(
		s.config.Port,
		s.config.TLS.Port,
		s.config.TLS.Key,
		s.config.TLS.Cert,
	)

	if err != nil {
		return err
	}

	s.Server.MaxConns = s.config.MaxConns
	s.Server.MaxConcurrentTLSHandshakes = s.config.MaxConcurrentTLSHandshakes
	s.Server.Handler = s.config.Handler()

	return s.Server.Run()
}

func (s *Service) Shutdown(os.Signal) {
	if s.Server != nil {
		s.Server.Stop()
	}
}
