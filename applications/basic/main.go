package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting test app")

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "success")
	}))
}
