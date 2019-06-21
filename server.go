package server

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/remerge/cue"
)

type Server struct {
	Id                         string
	Port                       int
	TlsPort                    int         // revive:disable:var-naming
	TlsConfig                  *tls.Config // revive:disable:var-naming
	MaxConns                   int64
	MaxConcurrentTLSHandshakes int64
	BufferSize                 int
	Timeout                    time.Duration // Service timeout. Default is 500ms

	Log     cue.Logger
	Handler Handler

	listener    *Listener
	tlsListener *Listener

	accepts                     metrics.Counter
	tooManyConns                metrics.Counter
	closes                      metrics.Counter
	numConns                    metrics.Counter
	numHandshakes               metrics.Counter
	tooManyConcurrentHandshakes metrics.Counter
	tlsErrors                   metrics.Counter
}

func NewServer(port int) (server *Server, err error) {
	server = &Server{
		Id:         fmt.Sprintf("server:%d", port),
		Port:       port,
		BufferSize: 32768,
		Timeout:    500 * time.Millisecond,
	}

	server.Log = cue.NewLogger(server.Id)
	server.Log.Infof("new server on port %d", port)

	server.accepts = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d accept", port), nil)
	server.tooManyConns = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d too_many_connection", port), nil)
	server.closes = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d close", port), nil)
	server.numConns = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d connection", port), nil)
	server.numHandshakes = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d handshakes", port), nil)
	server.tooManyConcurrentHandshakes = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d too_many_concurrent_handshakes", port), nil)
	server.tlsErrors = metrics.GetOrRegisterCounter(fmt.Sprintf("rex_server,port=%d tls_error", port), nil)

	return server, nil
}

func NewServerWithTLS(port int, tlsPort int, key string, cert string) (server *Server, err error) {
	server, err = NewServer(port)
	if err != nil {
		return nil, err
	}

	if tlsPort < 1 {
		return server, nil
	}

	server.Log.Infof("using TLS key=%v cert=%v", key, cert)
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	server.TlsConfig = &tls.Config{Certificates: []tls.Certificate{pair}}
	server.TlsPort = tlsPort

	return server, nil
}

func (server *Server) HasTLS() bool {
	return server.TlsPort > 0 && server.TlsConfig != nil
}

func (server *Server) Listen() (err error) {
	server.listener, err = NewListener(server.Port)
	if err != nil {
		return err
	}

	if server.HasTLS() {
		server.tlsListener, err = NewTlsListener(server.TlsPort, server.TlsConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (server *Server) Run() error {
	if err := server.Listen(); err != nil {
		return err
	}

	server.Serve()
	if server.HasTLS() {
		server.ServeTLS()
	}

	return nil
}

func (server *Server) Stop() {
	if server == nil {
		return
	}

	if server.HasTLS() && server.tlsListener != nil {
		server.Log.Infof("shutting down TLS listener")
		server.tlsListener.Stop()
		server.Log.Infof("waiting for requests to finish")
		server.tlsListener.Wait()
	}

	if server.listener != nil {
		server.Log.Infof("shutting down listener")
		server.listener.Stop()
		server.Log.Infof("waiting for requests to finish")
		server.listener.Wait()
	}

	server.waitForConnectionsToClose()
}

// StopDeadline is the hard deadline, after which Server.Stop() does no longer
// wait for connections to close and returns.
var StopDeadline = time.Minute

// waitForConnectionsToClose waits until all the handlers from listener and
// tlsListener are done and closed their connections.
func (server *Server) waitForConnectionsToClose() {
	deadline := time.NewTimer(StopDeadline)
	for {
		if server.numConns.Count() == 0 {
			server.Log.Infof("all requests finished")
			break
		}

		select {
		case <-deadline.C:
			server.Log.Warnf("not all requests finished after %v", StopDeadline)
			break
		case <-time.After(time.Millisecond):
		}
	}
}

func (server *Server) Serve() {
	server.listener.wg.Add(1)
	go func() {
		server.Log.Panic(server.listener.Run(server.acceptLoop), "could not run the listener")
	}()
}

func (server *Server) ServeTLS() {
	server.tlsListener.wg.Add(1)
	go func() {
		server.Log.Panic(server.tlsListener.Run(server.acceptLoop), "could not run the TLS listener")
	}()
}

func (server *Server) acceptLoop(listener *Listener) error {
	defer listener.Close()

	for {
		if listener.IsStopped() {
			return nil
		}

		conn, err := listener.Accept()
		if err != nil {
			if listener.IsStopped() {
				return nil
			}
			return err
		}
		server.accepts.Inc(1)

		// for cases of probe or blackhole connection
		if err := conn.SetDeadline(time.Now().Add(server.Timeout)); err != nil {
			_ = conn.Close()
			continue
		}

		if server.MaxConns > 0 && server.numConns.Count() > server.MaxConns {
			server.tooManyConns.Inc(1)
			_ = conn.Close()
			continue
		}

		if tlsConn, ok := conn.(*tls.Conn); ok {
			if server.MaxConcurrentTLSHandshakes > 0 && server.numHandshakes.Count() >= server.MaxConcurrentTLSHandshakes {
				server.tooManyConcurrentHandshakes.Inc(1)
				_ = tlsConn.Close()
				continue
			}
			// We need to increase the outstanding handshakes here otherwise we
			// accept way more than the limit due to the races with the new go routine.
			// This is safe as well as long as .Server() is called which does a numHandshakes.Dec(1) .
			server.numHandshakes.Inc(1)
		}

		go server.NewConnection(conn).Serve()
	}
}
