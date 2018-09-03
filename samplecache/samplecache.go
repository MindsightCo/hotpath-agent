package samplecache

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pkg/errors"
)

type HotpathSample struct {
	FnName string `json:"fnName"`
	NCalls int    `json:"nCalls"`
}

type RawSamples struct {
	mutex   *sync.RWMutex
	samples map[string]int
}

func NewRawSamples() *RawSamples {
	return &RawSamples{
		mutex:   new(sync.RWMutex),
		samples: make(map[string]int),
	}
}

func (s *RawSamples) Set(data map[string]int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for name, ncalls := range data {
		s.samples[name] += ncalls
	}
}

func (s *RawSamples) GetAll() []HotpathSample {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var hotpaths []HotpathSample

	for name, count := range s.samples {
		hotpaths = append(hotpaths, HotpathSample{name, count})
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

	s.samples = make(map[string]int)
}
