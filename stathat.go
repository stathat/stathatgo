// Copyright 2011 Numerotron Inc.
// Use of this source code is governed by an MIT-style license
// that can be found in the LICENSE file.
//
// Developed at www.stathat.com by Patrick Crosby
// Contact us on twitter with any questions:  twitter.com/stat_hat

// The stathat package makes it easy to post any values to your StatHat
// account.
package stathat

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type statKind int

const (
	_                = iota
	COUNTER statKind = iota
	VALUE
)

func (sk statKind) classicPath() string {
	switch sk {
	case COUNTER:
		return "/c"
	case VALUE:
		return "/v"
	}
	return ""
}

type apiKind int

const (
	_               = iota
	CLASSIC apiKind = iota
	EZ
)

const hostname = "api.stathat.com"

func (ak apiKind) path(sk statKind) string {
	switch ak {
	case EZ:
		return "/ez"
	case CLASSIC:
		return sk.classicPath()
	}
	return ""
}

type statReport struct {
	StatKey  string
	UserKey  string
	Value    float64
	statType statKind
	apiType  apiKind
}

var statReportChannel chan *statReport

var testingEnv = false

type testPost struct {
	url    string
	values url.Values
}

var testPostChannel chan *testPost
var Verbose = false

var done chan bool

func setTesting() {
	testingEnv = true
	testPostChannel = make(chan *testPost)
}

func init() {
	statReportChannel = make(chan *statReport, 100000)
	go processStats()
	done = make(chan bool)
}

// Using the classic API, posts a count to a stat.
func PostCount(statKey, userKey string, count int) error {
	statReportChannel <- newClassicStatCount(statKey, userKey, count)
	return nil
}

// Using the classic API, posts a count of 1 to a stat.
func PostCountOne(statKey, userKey string) error {
	return PostCount(statKey, userKey, 1)
}

// Using the classic API, posts a value to a stat.
func PostValue(statKey, userKey string, value float64) error {
	statReportChannel <- newClassicStatValue(statKey, userKey, value)
	return nil
}

// Using the EZ API, posts a count of 1 to a stat.
func PostEZCountOne(statName, ezkey string) error {
	return PostEZCount(statName, ezkey, 1)
}

// Using the EZ API, posts a count to a stat.
func PostEZCount(statName, ezkey string, count int) error {
	statReportChannel <- newEZStatCount(statName, ezkey, count)
	return nil
}

// Using the EZ API, posts a value to a stat.
func PostEZValue(statName, ezkey string, value float64) error {
	statReportChannel <- newEZStatValue(statName, ezkey, value)
	return nil
}

func newEZStatCount(statName, ezkey string, count int) *statReport {
	return &statReport{StatKey: statName,
		UserKey:  ezkey,
		Value:    float64(count),
		statType: COUNTER,
		apiType:  EZ}
}

func newEZStatValue(statName, ezkey string, value float64) *statReport {
	return &statReport{StatKey: statName,
		UserKey:  ezkey,
		Value:    value,
		statType: VALUE,
		apiType:  EZ}
}

func newClassicStatCount(statKey, userKey string, count int) *statReport {
	return &statReport{StatKey: statKey,
		UserKey:  userKey,
		Value:    float64(count),
		statType: COUNTER,
		apiType:  CLASSIC}
}

func newClassicStatValue(statKey, userKey string, value float64) *statReport {
	return &statReport{StatKey: statKey,
		UserKey:  userKey,
		Value:    value,
		statType: VALUE,
		apiType:  CLASSIC}
}

func (sr *statReport) values() url.Values {
	switch sr.apiType {
	case EZ:
		return sr.ezValues()
	case CLASSIC:
		return sr.classicValues()
	}

	return nil
}

func (sr *statReport) ezValues() url.Values {
	switch sr.statType {
	case COUNTER:
		return sr.ezCounterValues()
	case VALUE:
		return sr.ezValueValues()
	}
	return nil
}

func (sr *statReport) classicValues() url.Values {
	switch sr.statType {
	case COUNTER:
		return sr.classicCounterValues()
	case VALUE:
		return sr.classicValueValues()
	}
	return nil
}

func (sr *statReport) ezCommonValues() url.Values {
	result := make(url.Values)
	result.Set("stat", sr.StatKey)
	result.Set("ezkey", sr.UserKey)
	return result
}

func (sr *statReport) classicCommonValues() url.Values {
	result := make(url.Values)
	result.Set("key", sr.StatKey)
	result.Set("ukey", sr.UserKey)
	return result
}

func (sr *statReport) ezCounterValues() url.Values {
	result := sr.ezCommonValues()
	result.Set("count", sr.valueString())
	return result
}

func (sr *statReport) ezValueValues() url.Values {
	result := sr.ezCommonValues()
	result.Set("value", sr.valueString())
	return result
}

func (sr *statReport) classicCounterValues() url.Values {
	result := sr.classicCommonValues()
	result.Set("count", sr.valueString())
	return result
}

func (sr *statReport) classicValueValues() url.Values {
	result := sr.classicCommonValues()
	result.Set("value", sr.valueString())
	return result
}

func (sr *statReport) valueString() string {
	return strconv.FormatFloat(sr.Value, 'g', -1, 64)
}

func (sr *statReport) path() string {
	return sr.apiType.path(sr.statType)
}

func (sr *statReport) url() string {
	return fmt.Sprintf("http://%s%s", hostname, sr.path())
}

func processStats() {
	for {
		sr, ok := <-statReportChannel

		if !ok {
			done <- true
			break
		}

		if Verbose {
			log.Printf("posting stat to stathat: %s, %v", sr.url(), sr.values())
		}

		if testingEnv {
			testPostChannel <- &testPost{sr.url(), sr.values()}
			continue
		}

		r, err := http.PostForm(sr.url(), sr.values())
		if err != nil {
			log.Printf("error posting stat to stathat: %s", err)
			continue
		}

		if Verbose {
			body, _ := ioutil.ReadAll(r.Body)
			log.Printf("stathat post result: %s", body)
		}

		r.Body.Close()
	}
}

// Wait for all stats to be sent, or until timeout. Useful for simple command-
// line apps to defer a call to this in main()
func WaitUntilFinished(timeout time.Duration) bool {
	close(statReportChannel)
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
	return false
}
