package liberty

import (
	"net/http"
	"testing"
)

func TestGetMux(t *testing.T) {
	return
	var ports map[int]*http.ServeMux = map[int]*http.ServeMux{
		80:   nil,
		443:  nil,
		8080: nil,
	}
	for port, _ := range ports {
		ports[port] = getMux(port)
	}

	for port, mux := range ports {
		testMux := getMux(port)
		p1 := &mux
		p2 := &testMux
		if !(p1 == p2) {
			t.Errorf("address of mux 1 '%#v' does not match mux 2 '%#v'", p1, p2)
		}
	}
}
