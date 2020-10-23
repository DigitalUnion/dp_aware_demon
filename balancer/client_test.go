package balancer

import (
	"testing"
)

func TestBalancer(t *testing.T) {
	urls := []string{"http://192.168.1.24:8080/q", "http://192.168.1.79:8080/q"}
	bl, err := NewRoundrobinFromURL(urls...)
	if err != nil {
		t.Logf("get balancer error:%v", err)
	}
	client := NewClient(bl)
	resp, err := client.Get("http://10.0.0.1:8080/q?cid=test")
	if err != nil {
		t.Logf("get response error%v", err)
	}
	t.Logf("status:%s", resp.Status)

}
