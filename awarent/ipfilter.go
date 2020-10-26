package awarent

import (
	"net"
	"sync"
)

//FilterOptions for IPFilter. Allow/Block setting
type FilterOptions struct {
	//explicity allowed IPs
	AllowedIPs     []string     `yaml:"allowed"`
	BlockedIPs     []string     `yaml:"blocked"`
	URLPath        string       `yaml:"urlPath"`
	URLParam       string       `yaml:"urlParam"`
	AuthorizedIPs  []Authorized `yaml:"authorized"`
	BlockByDefault bool         `yaml:"blockedDefault"`
}

type Authorized struct {
	Resource string   `yaml:"resource"`
	IPS      []string `yaml:"ips"`
}

//Filter filter struct
type Filter struct {
	opts           FilterOptions
	mut            sync.RWMutex
	defaultAllowed bool
	allowedIPs     map[string]bool
	urlPath        string
	urlParam       string
	blockedIPs     map[string]bool
	authorizedIPs  map[string][]string
}

var ipfilter *Filter

//New new ipfilter
func New(opts FilterOptions) *Filter {

	f := &Filter{
		opts:           opts,
		allowedIPs:     map[string]bool{},
		blockedIPs:     map[string]bool{},
		authorizedIPs:  map[string][]string{},
		defaultAllowed: !opts.BlockByDefault,
	}
	f.urlParam = opts.URLParam
	f.urlPath = opts.URLPath

	for _, ip := range opts.AllowedIPs {
		f.allowIP(ip)
	}
	for _, ip := range opts.BlockedIPs {
		f.blockIP(ip)
	}
	for _, authrozied := range opts.AuthorizedIPs {
		for _, ip := range authrozied.IPS {
			f.authorizeIP(ip, authrozied.Resource)
		}

	}
	return f
}

func updateIPFilter(opts FilterOptions) {
	if ipfilter == nil {
		ipfilter = &Filter{
			opts:           opts,
			mut:            sync.RWMutex{},
			allowedIPs:     map[string]bool{},
			blockedIPs:     map[string]bool{},
			authorizedIPs:  map[string][]string{},
			defaultAllowed: !opts.BlockByDefault,
		}
	}

	ipfilter.urlParam = opts.URLParam
	ipfilter.urlPath = opts.URLPath
	for k := range ipfilter.allowedIPs {
		delete(ipfilter.allowedIPs, k)
	}
	for _, ip := range opts.AllowedIPs {
		ipfilter.allowIP(ip)
	}
	for k := range ipfilter.blockedIPs {
		delete(ipfilter.blockedIPs, k)
	}
	for _, ip := range opts.BlockedIPs {
		ipfilter.blockIP(ip)
	}

	for k := range ipfilter.authorizedIPs {
		delete(ipfilter.authorizedIPs, k)
	}
	for _, authrozied := range opts.AuthorizedIPs {
		for _, ip := range authrozied.IPS {
			ipfilter.authorizeIP(ip, authrozied.Resource)
		}

	}
}

//allowIP  settting allow ip address
func (f *Filter) allowIP(ip string) bool {
	if ip := net.ParseIP(ip); ip != nil {
		f.mut.Lock()
		f.allowedIPs[ip.String()] = true
		f.mut.Unlock()
		return true
	}
	return false
}

//blockIP setting block ip address
func (f *Filter) blockIP(ip string) bool {
	if ip := net.ParseIP(ip); ip != nil {
		f.mut.Lock()
		f.blockedIPs[ip.String()] = true
		f.mut.Unlock()
		return true
	}
	return false
}

//authorizeIP settting service authorized ip address
func (f *Filter) authorizeIP(ip string, identity string) bool {
	if ip := net.ParseIP(ip); ip != nil && len(identity) > 0 {
		f.mut.Lock()
		val, ok := f.authorizedIPs[identity]
		if ok {
			val = append(val, ip.String())
			f.authorizedIPs[identity] = val
		} else {
			val = make([]string, 0)
			val = append(val, ip.String())
			f.authorizedIPs[identity] = val
		}
		f.mut.Unlock()
		return true
	}
	return false
}

func (f *Filter) Allowed(ip string) bool {
	if ip == "" {
		return f.defaultAllowed
	}
	allowed, ok := f.allowedIPs[ip]
	if ok {
		return allowed
	}
	blocked, ok := f.blockedIPs[ip]
	if ok {
		return !blocked
	}
	return f.defaultAllowed
}

// func (f *IPFilter) Blocked(ip string) bool {
// 	if ip == "" {
// 		return false
// 	}
// 	blocked, ok := f.blockedIPs[ip]
// 	if ok {
// 		return blocked
// 	}
// 	return false
// }

func (f *Filter) Authorized(ip string, param string) bool {
	if len(param) == 0 {
		return false
	}

	if ips, ok := f.authorizedIPs[param]; ok {
		for _, val := range ips {
			if ip == val {
				return true
			}
		}
	}

	return false
}
