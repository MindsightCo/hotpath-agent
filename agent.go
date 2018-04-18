package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/machinebox/graphql"
)

var (
	host     string
	port     int
	server   string
	cacheLen int
	client   *graphql.Client
)

func init() {
	flag.StringVar(&host, "host", "", "Address to bind server to")
	flag.IntVar(&port, "port", 8000, "Port to listen on")
	flag.IntVar(&cacheLen, "cache", 100, "Number requests to cache before sending samples")
	flag.StringVar(&server, "server", "https://api.mindsight.io", "URL of API server")
}

type hotpathSample struct {
	FnName string
	NCalls int
}

var mutation string = `
mutation ($samples: [HotpathSample!]!) {
	addHotpath(hotpaths: $samples)
}
`

func sendSamples(samples map[string]int) error {
	var (
		response int
		params   []hotpathSample
	)

	for name, count := range samples {
		params = append(params, hotpathSample{name, count})
	}

	req := graphql.NewRequest(mutation)
	req.Var("samples", params)
	return client.Run(context.Background(), req, &response)
}

func main() {
	flag.Parse()

	client = graphql.NewClient(server)
	samples := make(map[string]int)
	count := 0

	http.HandleFunc("/samples/", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]int
		defer r.Body.Close()

		if r.Method != "POST" {
			http.Error(w, "only POST allowed for /samples/", http.StatusNotFound)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			msg := fmt.Sprintf("invalid json: %s", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		for name, ncalls := range data {
			samples[name] += ncalls
		}

		log.Println(samples)

		count += 1
		if count > cacheLen {
			if err := sendSamples(samples); err != nil {
				log.Println(err)
			} else {
				samples = make(map[string]int)
			}
		}

		w.WriteHeader(http.StatusCreated)
	})

	bind := fmt.Sprintf("%s:%d", host, port)
	log.Fatal(http.ListenAndServe(bind, nil))
}
