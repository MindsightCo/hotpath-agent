package samplecache

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pkg/errors"
)

type Hotpath struct {
	FnName string `json:"fnName"`
	NCalls int    `json:"nCalls"`
}

type HotpathSample struct {
	ProjectName string    `json:"projectName"`
	Environment string    `json:"environment,omitempty"`
	Hotpaths    []Hotpath `json:"hotpaths"`
}

type sampleKey struct {
	fn      string
	project string
	env     string
}

type RawSamples struct {
	mutex   *sync.RWMutex
	samples map[sampleKey]int
}

func NewRawSamples() *RawSamples {
	return &RawSamples{
		mutex:   new(sync.RWMutex),
		samples: make(map[sampleKey]int),
	}
}

func (s *RawSamples) Set(data map[string]int, project, environment string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for name, ncalls := range data {
		key := sampleKey{
			project: project,
			env:     environment,
			fn:      name,
		}
		s.samples[key] += ncalls
	}

	log.Println(data)
}

type projectEnvKey struct {
	project string
	env     string
}

func (s *RawSamples) groupByProjectEnv() map[projectEnvKey]*HotpathSample {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	perProjectEnv := make(map[projectEnvKey]*HotpathSample)

	for key, count := range s.samples {
		peKey := projectEnvKey{project: key.project, env: key.env}

		if _, present := perProjectEnv[peKey]; !present {
			perProjectEnv[peKey] = &HotpathSample{
				ProjectName: peKey.project,
				Environment: peKey.env,
			}
		}

		s := perProjectEnv[peKey]
		s.Hotpaths = append(s.Hotpaths, Hotpath{FnName: key.fn, NCalls: count})
	}

	return perProjectEnv
}

func (s *RawSamples) GetAll() []*HotpathSample {
	perProjectEnv := s.groupByProjectEnv()

	var hotpaths []*HotpathSample

	for _, h := range perProjectEnv {
		hotpaths = append(hotpaths, h)
	}

	return hotpaths
}

func (s *RawSamples) Dump() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	prettyPrint, err := json.MarshalIndent(s.samples, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to format samples to json")
	}

	log.Println("TESTMODE | Samples accumulated thus far (not sending to server):", string(prettyPrint))
	return nil
}

func (s *RawSamples) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.samples = make(map[sampleKey]int)
}
