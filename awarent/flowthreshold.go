package awarent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type traffic struct {
	countMap map[string]int64
	mutex    *sync.RWMutex
	client   *http.Client
	name     string
}

func newTraffic(name string) *traffic {
	return &traffic{
		countMap: make(map[string]int64),
		mutex:    new(sync.RWMutex),
		name:     name,
	}
}

func (t *traffic) add(resource string, count int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if value, ok := t.countMap[resource]; ok {
		t.countMap[resource] = value + count
	} else {
		t.countMap[resource] = count
	}
}

func (t *traffic) reset() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.countMap = make(map[string]int64)
}

func (t *traffic) record() {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	data, err := json.Marshal(t.countMap)
	if err == nil {
		t.send(string(data))
	}
	t.reset()
}

func (t *traffic) send(data string) {
	defer func() {
		recover()
	}()

	if t.client == nil {
		t.client = new(http.Client)
	}
	reqBody, err := json.Marshal(map[string]string{"data": data, "resource": t.name})
	if err != nil {
		return
	}
	t.client.Post("http://127.0.0.1:8181/q", "application/json", bytes.NewReader(reqBody))
}

func (a *Awarent) DynamicRecord() {
	for range time.Tick(time.Minute) {
		a.count.record()
	}
}
