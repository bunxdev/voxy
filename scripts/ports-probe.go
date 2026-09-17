// Test fixture only: HTTP on 8080/8081 and UDP echo on 8082.
package main

import (
	"fmt"
	"net"
	"net/http"
)

func main() {
	u, e := net.ListenPacket("udp", "0.0.0.0:8082")
	if e != nil {
		panic(e)
	}
	go func() {
		b := make([]byte, 1024)
		for {
			n, a, e := u.ReadFrom(b)
			if e != nil {
				panic(e)
			}
			u.WriteTo(b[:n], a)
		}
	}()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "voxy-ports-ok\n") })
	go func() { panic(http.ListenAndServe("0.0.0.0:8081", handler)) }()
	panic(http.ListenAndServe("0.0.0.0:8080", handler))
}
