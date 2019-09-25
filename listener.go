package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/remerge/cue"
)

type Listener struct {
	net.Listener
	wg      sync.WaitGroup
	log     cue.Logger
	stopped int32 // atomic bool
}

func NewListener(port int, listenConfig *net.ListenConfig) (listener *Listener, err error) {
	listener = &Listener{}

	listener.log = cue.NewLogger(fmt.Sprintf("listener:%d", port))
	listener.log.Infof("start listen on port %d", port)

	listener.Listener, err = listenConfig.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	return listener, nil
}

// revive:disable:var-naming
func NewTlsListener(port int, tlsConfig *tls.Config, listenConfig *net.ListenConfig) (listener *Listener, err error) {
	listener, err = NewListener(port, listenConfig)
	if err != nil {
		return nil, err
	}

	listener.Listener = tls.NewListener(listener.Listener, tlsConfig)
	return listener, nil
}

func (listener *Listener) Accept() (conn net.Conn, err error) {
	return listener.Listener.Accept()
}

func (listener *Listener) Run(callback func(*Listener) error) error {
	defer listener.wg.Done()
	return callback(listener)
}

func (listener *Listener) Stop() {
	atomic.StoreInt32(&listener.stopped, 1)
	_ = listener.Listener.Close()
}

func (listener *Listener) IsStopped() bool {
	return atomic.LoadInt32(&listener.stopped) > 0
}

func (listener *Listener) Wait() {
	listener.wg.Wait()
}
