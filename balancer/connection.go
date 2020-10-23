package balancer

import (
	"net"
	"net/url"
	"sync"
	"time"
)

type Connection interface {
	URL() *url.URL
	IsBroken() bool
}

type HttpConnection struct {
	sync.Mutex
	url               *url.URL
	broken            bool
	heartbeatDuration time.Duration
	heartbeatStop     chan bool
}

func NewHttpConnection(url *url.URL) *HttpConnection {
	c := &HttpConnection{
		url:               url,
		heartbeatDuration: 5 * time.Second,
		heartbeatStop:     make(chan bool),
	}
	c.checkBroken()
	go c.heartbeat()
	return c
}

func (c *HttpConnection) Close() error {
	c.Lock()
	defer c.Unlock()
	c.heartbeatStop <- true
	c.broken = false
	return nil
}

func (c *HttpConnection) HeartbeatDuration(d time.Duration) *HttpConnection {
	c.Lock()
	defer c.Unlock()
	c.heartbeatStop <- true
	c.broken = false
	c.heartbeatDuration = d
	go c.heartbeat()
	return c
}

func (c *HttpConnection) heartbeat() {
	ticker := time.NewTicker(c.heartbeatDuration)
	for {
		select {
		case <-ticker.C:
			c.checkBroken()
		case <-c.heartbeatStop:
			return
		}
	}
}

func (c *HttpConnection) checkBroken() {
	c.Lock()
	defer c.Unlock()
	conn, err := net.DialTimeout("tcp", c.url.Host, time.Second*5)
	if err != nil {
		c.broken = true
		return
	}
	_ = conn.Close()
	c.broken = false
}

func (c *HttpConnection) URL() *url.URL {
	return c.url
}

func (c *HttpConnection) IsBroken() bool {
	return c.broken
}
