//   Copyright © 2018, Oracle and/or its affiliates.  All rights reserved.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type citiesResponse struct {
	Cities []string `json:"cities"` // Cities capitalised to export it, otherwise json encoder will ignore it.
}

func cityHandler(res http.ResponseWriter, req *http.Request) {
	cities := citiesResponse{
		Cities: []string{"Amsterdam", "Berlin", "New York", "San Francisco", "Tokyo"}}

	res.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(res).Encode(cities)
}

func defaultHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.Write([]byte("Hello World from docker-push-image!"))
}

func main() {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/cities.json", cityHandler)
	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal("Unable to listen on port 5000 : ", err)
	}
}
