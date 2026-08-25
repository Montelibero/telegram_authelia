package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		for _, name := range []string{"Remote-User", "Remote-Email", "Remote-Groups", "Remote-Name"} {
			_, _ = fmt.Fprintf(w, "%s: %s\n", name, r.Header.Get(name))
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
