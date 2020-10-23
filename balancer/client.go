package balancer

import "net/http"

// NewClient return a http client that applies a roundrobin load
// balance between serveral HTTP servers
func NewClient(b Balancer) *http.Client {
	return &http.Client{
		Transport: &Transport{balancer: b},
	}
}
