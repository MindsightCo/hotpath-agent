package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/MindsightCo/hotpath-agent/msclient"
	"github.com/MindsightCo/hotpath-agent/samplecache"
	"github.com/ereyes01/go-auth0-grant"
	"github.com/pkg/errors"
)

var (
	host     string
	port     int
	server   string
	cacheLen int
	testMode bool
	client   *http.Client
)

const (
	CREDS_AUDIENCE     = "https://api.mindsight.io/"
	DEFAULT_API_SERVER = "https://api.mindsight.io/query"
	AUTH0_TOKEN_URL    = "https://mindsight.auth0.com/oauth/token/"
)

func init() {
	flag.StringVar(&host, "host", "", "Address to bind server to")
	flag.IntVar(&port, "port", 8000, "Port to listen on")
	flag.IntVar(&cacheLen, "cache", 5, "Number requests to cache before sending samples")
	flag.StringVar(&server, "server", DEFAULT_API_SERVER, "URL of API server")
	flag.BoolVar(&testMode, "test", false, "Enable test mode, does not attempt to send data")
	client = &http.Client{}
}

type dataSample struct {
	ProjectName string                      `json:"projectName"`
	Environment string                      `json:"environment"`
	Hotpaths    []samplecache.HotpathSample `json:"hotpaths"`
}

var mutation string = `
mutation ($sample: DataSample!) {
	collectData(sample: $sample)
}
`

func sendSamples(projectName, environment string, samples *samplecache.RawSamples, grant auth0grant.Grant) error {
	sample := dataSample{ProjectName: projectName, Environment: environment}

	sample.Hotpaths = samples.GetAll()

	gql := msclient.GraphqlRequest{
		Query: mutation,
		Variables: map[string]interface{}{
			"sample": sample,
		},
	}

	if _, err := msclient.APIRequest(server, &gql, grant); err != nil {
		return errors.Wrap(err, "send hotpath samples")
	}

	return nil
}

func initGrant(testMode bool) (auth0grant.Grant, error) {
	if testMode {
		return nil, nil
	}

	credRequest := &auth0grant.CredentialsRequest{
		ClientID:     os.Getenv("MINDSIGHT_CLIENT_ID"),
		ClientSecret: os.Getenv("MINDSIGHT_CLIENT_SECRET"),
		Audience:     CREDS_AUDIENCE,
		GrantType:    auth0grant.CLIENT_CREDS_GRANT_TYPE,
	}

	if credRequest.ClientID == "" || credRequest.ClientSecret == "" {
		return nil, errors.New("Must supply env variables MINDSIGHT_CLIENT_ID and MINDSIGHT_CLIENT_SECRET")
	}

	grant := auth0grant.NewGrant(AUTH0_TOKEN_URL, credRequest)

	// test the token
	if _, err := grant.GetAccessToken(); err != nil {
		return nil, errors.Wrap(err, "testing credentials")
	}

	return grant, nil
}

func main() {
	flag.Parse()

	grant, err := initGrant(testMode)
	if err != nil {
		err = errors.Wrap(err, "Mindsight agent: fail init grant")
		log.Fatal(err)
	}

	log.Println("Starting Mindsight agent...")

	samples := samplecache.NewRawSamples()
	count := 0

	http.HandleFunc("/samples/", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]int
		defer r.Body.Close()

		query := r.URL.Query()
		projectName := query.Get("project")
		environment := query.Get("environment")

		if projectName == "" {
			http.Error(w, "must specify ``project'' query parameter", http.StatusBadRequest)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "only POST allowed for /samples/", http.StatusNotFound)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			msg := fmt.Sprintf("invalid json: %s", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		samples.Set(data)

		count += 1
		if count > cacheLen {
			var err error

			if testMode {
				err = samples.Dump()
			} else {
				err = sendSamples(projectName, environment, samples, grant)
			}

			if err != nil {
				log.Println(err)
			} else {
				samples.Clear()
				count = 0
			}
		}

		w.WriteHeader(http.StatusCreated)
	})

	bind := fmt.Sprintf("%s:%d", host, port)
	log.Fatal(http.ListenAndServe(bind, nil))
}
