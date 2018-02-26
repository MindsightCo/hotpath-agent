package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
)

var (
	host string
	port int
)

func init() {
	flag.StringVar(&host, "host", "", "Address to bind server to")
	flag.IntVar(&port, "port", 8000, "Port to listen on")
}

func main() {
	flag.Parse()

	http.HandleFunc("/samples/", func(w http.ResponseWriter, r *http.Request) {
		var data []interface{}
		defer r.Body.Close()

		if r.Method != "POST" {
			http.Error(w, "only POST for /samples/", http.StatusNotFound)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			msg := fmt.Sprintf("invalid json: %s", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		pretty, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			msg := fmt.Sprintf("failed to prettify: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		log.Println(string(pretty))

		w.WriteHeader(http.StatusCreated)
	})

	bind := fmt.Sprintf("%s:%d", host, port)
	log.Fatal(http.ListenAndServe(bind, nil))
}
