package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/pkg/errors"
)

var (
	host     string
	port     int
	server   string
	cacheLen int
	client   *http.Client
)

func init() {
	flag.StringVar(&host, "host", "", "Address to bind server to")
	flag.IntVar(&port, "port", 8000, "Port to listen on")
	flag.IntVar(&cacheLen, "cache", 100, "Number requests to cache before sending samples")
	flag.StringVar(&server, "server", "https://api.mindsight.io", "URL of API server")
	client = &http.Client{}
}

type hotpathSample struct {
	FnName string `json:"fnName"`
	NCalls int    `json:"nCalls"`
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type graphqlErrLoc struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type graphqlError struct {
	Message   string          `json:"message"`
	Locations []graphqlErrLoc `json:"locations"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
}

var mutation string = `
mutation ($samples: [HotpathSample!]!) {
	addHotpath(hotpaths: $samples)
}
`

func sendSamples(samples map[string]int) error {
	var (
		gqlResp graphqlResponse
		params  []hotpathSample
	)

	for name, count := range samples {
		params = append(params, hotpathSample{name, count})
	}

	gql := graphqlRequest{
		Query: mutation,
		Variables: map[string]interface{}{
			"samples": params,
		},
	}

	payload, err := json.Marshal(gql)
	if err != nil {
		return errors.Wrap(err, "json-marshal graphql request payload")
	}

	req, err := http.NewRequest("POST", server, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrap(err, "create new request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "do http request")
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return errors.Wrap(err, "decode gql response body")
	}

	if len(gqlResp.Errors) > 0 {
		return errors.New("graphql error: " + gqlResp.Errors[0].Message)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New("bad http status code: " + resp.Status)
	}

	return nil
}

func main() {
	flag.Parse()

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

		log.Println(count, samples)

		count += 1
		if count > cacheLen {
			if err := sendSamples(samples); err != nil {
				log.Println(err)
			} else {
				samples = make(map[string]int)
				count = 0
			}
		}

		w.WriteHeader(http.StatusCreated)
	})

	bind := fmt.Sprintf("%s:%d", host, port)
	log.Fatal(http.ListenAndServe(bind, nil))
}
