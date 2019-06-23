// Serve static files from current working directory
//  it will start at port 8080 if port is being used it will try next one
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	port := 8080
	for {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "err opening port", err)
			port++
			continue
		}
		fmt.Printf("Listening at %s\n", addr)
		log.Fatal(http.Serve(listener, logger(http.FileServer(http.Dir(".")))))
	}
}

func logger(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	}
}
