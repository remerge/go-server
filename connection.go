package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/remerge/cue"
)

type Connection struct {
	net.Conn
	Server      *Server
	LimitReader io.LimitedReader
	Buffer      bufio.ReadWriter
	closeMutex  sync.Mutex
}

// NoLimit is an effective infinite upper bound for io.LimitedReader
const NoLimit int64 = (1 << 63) - 1

var connectionPool sync.Pool

func (server *Server) NewConnection(conn net.Conn) *Connection {
	c := newConnection()
	c.Conn = conn
	c.Server = server

	c.LimitReader.R = conn
	c.LimitReader.N = NoLimit

	br := newBufioReader(&c.LimitReader, server.BufferSize)
	bw := newBufioWriter(conn, server.BufferSize)
	c.Buffer.Reader = br
	c.Buffer.Writer = bw

	c.Server.numConns.Inc(1)

	c.Server.connectionsMutex.Lock()
	defer c.Server.connectionsMutex.Unlock()
	c.Server.connections[c] = struct{}{}
	return c
}

func newConnection() *Connection {
	if v := connectionPool.Get(); v != nil {
		return v.(*Connection)
	}
	return &Connection{}
}

func putConnection(c *Connection) {
	c.Server.numConns.Dec(1)
	c.Server.connectionsMutex.Lock()
	delete(c.Server.connections, c)
	c.Server.connectionsMutex.Unlock()

	c.Conn = nil
	c.Server = nil
	c.LimitReader.R = nil
	c.LimitReader.N = 0

	if c.Buffer.Reader != nil {
		putBufioReader(c.Buffer.Reader)
		c.Buffer.Reader = nil
	}

	if c.Buffer.Writer != nil {
		putBufioWriter(c.Buffer.Writer)
		c.Buffer.Writer = nil
	}

	connectionPool.Put(c)
}

var (
	bufioReaderPool sync.Pool
	bufioWriterPool sync.Pool
)

func newBufioReader(r io.Reader, size int) *bufio.Reader {
	if v := bufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReaderSize(r, size)
}

func putBufioReader(br *bufio.Reader) {
	br.Reset(nil)
	bufioReaderPool.Put(br)
}

func newBufioWriter(w io.Writer, size int) *bufio.Writer {
	if v := bufioWriterPool.Get(); v != nil {
		bw := v.(*bufio.Writer)
		bw.Reset(w)
		return bw
	}
	return bufio.NewWriterSize(w, size)
}

func putBufioWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	bufioWriterPool.Put(bw)
}

func trimStringFromSym(s string) string {
	if idx := strings.Index(s, ":"); idx != -1 {
		return s[:idx]
	}
	return s
}

func (c *Connection) Serve() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("unhandled panic: %v\n", err)
			debug.PrintStack()

			c.Server.Log.WithFields(cue.Fields{
				"person_id": c.Conn.RemoteAddr().String(),
			}).Panic(err, "unhandled server connection error")
		}
		c.Close()
	}()

	remoteAddr := trimStringFromSym(c.Conn.RemoteAddr().String())
	strs := strings.Split(remoteAddr, ".")
	remoteAddr = strs[0] + "." + strs[1]

	if tlsConn, ok := c.Conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			c.Server.Log.Warnf("Connection: %s", remoteAddr)
			c.Server.Log.Warnf("Connection state: %v", tlsConn.ConnectionState())
			c.Server.errorsMutex.Lock()
			goodBad := c.Server.errors[remoteAddr]
			goodBad.bad++
			c.Server.errors[remoteAddr] = goodBad
			c.Server.errorsMutex.Unlock()
			c.Server.tlsErrors.Inc(1)
			c.Server.numHandshakes.Dec(1)
			return
		}
		c.Server.errorsMutex.Lock()
		goodBad := c.Server.errors[remoteAddr]
		goodBad.goodTls++
		c.Server.errors[remoteAddr] = goodBad
		c.Server.errorsMutex.Unlock()
		c.Server.numHandshakes.Dec(1)
	}

	// reset deadline before handle
	if err := c.Conn.SetDeadline(time.Time{}); err != nil {
		return
	}

	c.Server.Handler.Handle(c)
}

func (c *Connection) closeInternal() {
	// prevent double close
	if c.Conn == nil {
		return
	}

	if c.Server != nil {
		c.Server.closes.Inc(1)
	}

	if err := c.Conn.SetDeadline(time.Now().Add(c.Server.Timeout)); err != nil {
		_ = c.Conn.Close()
		return
	}

	// flush write buffer before close
	if c.Buffer.Writer != nil {
		_ = c.Buffer.Writer.Flush()
	}
	_ = c.Conn.Close()
}

// Close - closes the underlying connection and puts it back in the pool
// IMPORTANT: this should NEVER be called twice as it is not go routine safe:
// The connection is put back in the pool and might be taken and reinitialized by
// another go routine. If Close() is called a second time it will modify the connection
// that is potentially already in use in a different go routine
func (c *Connection) Close() {
	c.closeMutex.Lock()
	c.closeInternal()
	c.closeMutex.Unlock()
	// put connection back into pool
	putConnection(c)
}
