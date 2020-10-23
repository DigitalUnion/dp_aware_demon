package balancer

import (
	"errors"
	"net/url"
	"sync"
)

// Roundrobin load balancer
type Roundrobin struct {
	sync.Mutex
	conns []Connection
	idx   int
}

// NewRoundrobin new round robin with connection
func NewRoundrobin(conns ...Connection) (*Roundrobin, error) {
	b := &Roundrobin{
		conns: make([]Connection, 0),
	}
	if len(conns) > 0 {
		b.conns = append(b.conns, conns...)
	}
	return b, nil
}

// NewRoundrobinFromURL creates a new round-robin balancer from the given
// urls. it returns error if any of URLs is invalid
func NewRoundrobinFromURL(urls ...string) (*Roundrobin, error) {
	b := &Roundrobin{
		conns: make([]Connection, 0),
	}
	for _, rawurl := range urls {
		if u, err := url.Parse(rawurl); err != nil {
			return nil, err
		} else {
			b.conns = append(b.conns, NewHttpConnection(u))
		}
	}
	return b, nil
}

//ErrNoConn no connection error
var ErrNoConn = errors.New("no connection")

// Get connection from pool
func (r *Roundrobin) Get() (Connection, error) {
	r.Lock()
	defer r.Unlock()
	if len(r.conns) == 0 {
		return nil, ErrNoConn
	}
	var conn Connection
	for i := 0; i < len(r.conns); i++ {
		candidate := r.conns[r.idx]
		r.idx = (r.idx + 1) % len(r.conns)
		if !candidate.IsBroken() {
			conn = candidate
			break
		}
	}
	if conn == nil {
		return nil, ErrNoConn
	}
	return conn, nil
}

// Connections return connections
func (r *Roundrobin) Connections() []Connection {
	r.Lock()
	defer r.Unlock()
	conns := make([]Connection, len(r.conns))
	for i, c := range r.conns {
		if oc, ok := c.(*HttpConnection); ok {
			cr := &simpleConn{
				url:    oc.URL(),
				broken: oc.IsBroken(),
			}
			conns[i] = cr
		}
	}
	return conns
}

type simpleConn struct {
	url    *url.URL
	broken bool
}

func (c *simpleConn) URL() *url.URL  { return c.url }
func (c *simpleConn) IsBroken() bool { return c.broken }
