package server

import (
	"os"

	"github.com/remerge/go-service"
	"github.com/remerge/go-service/registry"
	"github.com/spf13/cobra"
)

type ServerConfig struct {
	DefaultPort int

	Port int
	TLS  struct {
		Port int
		Cert string
		Key  string
	}

	MaxConns                   int64
	MaxConcurrentTLSHandshakes int64

	CreateHandler func() Handler
}

type ClientServiceParams struct {
	registry.Params

	ServerConfig `registry:"lazy"`
}

type ClientService struct {
	Server *Server

	config ServerConfig
}

func RegisterService(r service.Registry) {
	r.Register(func(cmd *cobra.Command, p *ClientServiceParams) (*ClientService, error) {
		s := &ClientService{
			config: p.ServerConfig,
		}
		s.configureFlags(cmd)

		return s, nil
	})
}

func (s *ClientService) configureFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.IntVar(
		&s.config.Port,
		"server-port", s.config.DefaultPort,
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

func (s *ClientService) Init() error {
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
	s.Server.Handler = s.config.CreateHandler()

	return s.Server.Run()
}

func (s *ClientService) Shutdown(os.Signal) {
	if s.Server != nil {
		s.Server.Stop()
	}
}
