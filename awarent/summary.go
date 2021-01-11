package awarent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	sendLimit = 1000
)

var SMap *summaryMap

func init() {
	SMap = &summaryMap{
		sMap:   make(map[string]int64),
		lock:   new(sync.RWMutex),
		client: new(http.Client),
	}
	SMap.client.Timeout = time.Second * 1
}

type req struct {
	RuleId  string `json:"rule_id"`
	Cid     string `json:"cid"`
	Queries int64  `json:"queries"`
}

type summaryMap struct {
	sMap   map[string]int64
	lock   *sync.RWMutex
	client *http.Client
}

func (s summaryMap) add(ruleId, cid string, queries interface{}) {
	var value int64
	switch queries.(type) {
	case int:
		value = int64(queries.(int))
	case int64:
		value = int64(queries.(int))
	default:
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sMap[cid] += value
	if s.sMap[cid] >= sendLimit {
		go s.send(ruleId, cid, value)
		s.sMap[cid] = 0
	}
}

func (s summaryMap) send(ruleId, cid string, queries int64) {
	reqBody, err := json.Marshal(req{RuleId: ruleId, Cid: cid, Queries: queries})
	if err != nil {
		return
	}
	s.client.Post("http://172.17.130.223:8181/q", "application/json", bytes.NewReader(reqBody))
}
