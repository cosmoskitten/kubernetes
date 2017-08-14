/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

var (
	openAPIFile = flag.String("openapi", "", "File path to openapi-spec of Kubernetes. If not specifying, the openapi-spec is download from https://raw.githubusercontent.com/kubernetes/kubernetes/master/api/openapi-spec/swagger.json instead")
	restLog     = flag.String("restlog", "", "File path to REST API operation log of Kubernetes")
)

// Standard HTTP methods: https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md#path-item-object
func isHTTPMethod(method string) bool {
	methods := []string{"get", "put", "post", "delete", "options", "head", "patch"}
	for _, validMethod := range methods {
		if method == validMethod {
			return true
		}
	}
	return false
}

type apiData struct {
	Method string
	URL    string
}

type apiArray []apiData

var reOpenapi = regexp.MustCompile(`({\S+})`)

func parseOpenAPI(openapi string) apiArray {
	var decodeData interface{}
	var apisOpenapi apiArray

	if len(openapi) == 0 {
		url := "https://raw.githubusercontent.com/kubernetes/kubernetes/master/api/openapi-spec/swagger.json"
		resp, err := http.Get(url)
		if err != nil {
			log.Fatal(err)
		}
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(bytes, &decodeData); err != nil {
			log.Fatal(err)
		}
	} else {
		bytes, err := ioutil.ReadFile(openapi)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(bytes, &decodeData); err != nil {
			log.Fatal(err)
		}
	}
	for key, data := range decodeData.(map[string]interface{}) {
		if key != "paths" {
			continue
		}
		for apiURL, apiSpec := range data.(map[string]interface{}) {
			for apiMethod := range apiSpec.(map[string]interface{}) {
				if !isHTTPMethod(apiMethod) {
					continue
				}
				apiMethod := strings.ToUpper(apiMethod)
				api := apiData{
					Method: apiMethod,
					URL:    apiURL,
				}
				apisOpenapi = append(apisOpenapi, api)
			}
		}
	}
	return apisOpenapi
}

// Request: POST https://172.27.138.84:6443/api/v1/namespaces
var reAPILog = regexp.MustCompile(`Request: (\S+) (\S+)`)

func parseAPILog(restlog string) apiArray {
	var fp *os.File
	var apisLog apiArray
	var err error

	fp, err = os.Open(restlog)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, 4096)
	for line := ""; err == nil; line, err = reader.ReadString('\n') {
		result := reAPILog.FindSubmatch([]byte(line))
		if len(result) == 0 {
			continue
		}
		method := strings.ToUpper(string(result[1]))
		url := string(result[2])
		api := apiData{
			Method: method,
			URL:    url,
		}
		//TODO: Remove duplicated entries for speed
		apisLog = append(apisLog, api)
	}
	return apisLog
}

func main() {
	var found bool
	var numFound int
	var numNotFound int

	flag.Parse()
	if len(*restLog) == 0 {
		glog.Fatal("need to set '--restlog'")
	}

	apisOpenapi := parseOpenAPI(*openAPIFile)
	apisLogs := parseAPILog(*restLog)

	numFound = 0
	numNotFound = 0
	for _, openapi := range apisOpenapi {
		regURL := reOpenapi.ReplaceAllLiteralString(openapi.URL, `\S+`)
		reg := regexp.MustCompile(regURL)
		found = false
		for _, log := range apisLogs {
			if openapi.Method != log.Method {
				continue
			}
			if reg.MatchString(log.URL) {
				found = true
				numFound++
				break
			}
		}
		if found == false {
			fmt.Printf("The API(%s %s) is not found in e2e operation log.\n", openapi.Method, openapi.URL)
			numNotFound++
		}
	}
	fmt.Printf("All APIs: %d\n", len(apisOpenapi))
	fmt.Printf("numFound: %d\n", numFound)
	fmt.Printf("numNotFound: %d\n", numNotFound)
}
