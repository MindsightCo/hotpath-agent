package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/MindsightCo/hotpath-agent/auth"
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
	ProjectName string          `json:"projectName"`
	Environment string          `json:"environment"`
	Hotpaths    []hotpathSample `json:"hotpaths"`
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
mutation ($sample: DataSample!) {
	collectData(sample: $sample)
}
`

type rawSamples struct {
	mutex   *sync.RWMutex
	samples map[string]int
}

func NewRawSamples() *rawSamples {
	return &rawSamples{
		mutex:   new(sync.RWMutex),
		samples: make(map[string]int),
	}
}

func (s *rawSamples) Set(data map[string]int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for name, ncalls := range data {
		s.samples[name] += ncalls
	}
}

func (s *rawSamples) GetAll() []hotpathSample {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var hotpaths []hotpathSample

	for name, count := range s.samples {
		hotpaths = append(hotpaths, hotpathSample{name, count})
	}

	return hotpaths
}

func (s *rawSamples) Print() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	prettyPrint, err := json.MarshalIndent(s.samples, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to format samples to json")
	}

	log.Println("TESTMODE | Samples accumulated thus far (not sending to server):", string(prettyPrint))
	return nil
}

func (s *rawSamples) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.samples = make(map[string]int)
}

func sendSamples(projectName, environment string, samples *rawSamples, grant auth.Grant) error {
	var gqlResp graphqlResponse

	sample := dataSample{ProjectName: projectName, Environment: environment}

	accessToken, err := grant.GetAccessToken()
	if err != nil {
		return errors.Wrap(err, "get API access token")
	}

	sample.Hotpaths = samples.GetAll()

	gql := graphqlRequest{
		Query: mutation,
		Variables: map[string]interface{}{
			"sample": sample,
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
	req.Header.Set("Authorization", "bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "do http request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("response status: %s, body: %s", resp.Status, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Wrapf(err, "decode gql, response body: %s", string(body))
	}

	if len(gqlResp.Errors) > 0 {
		return errors.New("graphql error: " + gqlResp.Errors[0].Message)
	}

	return nil
}

func initGrant(testMode bool) (auth.Grant, error) {
	if testMode {
		return nil, nil
	}

	credRequest := &auth.CredentialsRequest{
		ClientID:     os.Getenv("MINDSIGHT_CLIENT_ID"),
		ClientSecret: os.Getenv("MINDSIGHT_CLIENT_SECRET"),
		Audience:     CREDS_AUDIENCE,
		GrantType:    auth.CLIENT_CREDS_GRANT_TYPE,
	}

	if credRequest.ClientID == "" || credRequest.ClientSecret == "" {
		return nil, errors.New("Must supply env variables MINDSIGHT_CLIENT_ID and MINDSIGHT_CLIENT_SECRET")
	}

	grant := auth.NewGrant(auth.AUTH0_TOKEN_URL, credRequest, nil)

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

	samples := NewRawSamples()
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

		log.Println(count, samples.samples)

		count += 1
		if count > cacheLen {
			var err error

			if testMode {
				err = samples.Print()
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
