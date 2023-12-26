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
	"log"
	"net/http"
	"os"
)

// envHandler handles queries of the form /env/foo
// foo is the name of an environment variable
// and this handler returns its value
func envHandler(res http.ResponseWriter, req *http.Request) {
	varname := req.URL.Path[len("/env/"):]
	value := os.Getenv(varname)
	res.Header().Set("Content-Type", "application/json; charset=utf-8")
	res.Write([]byte(value))
}

func defaultHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.Write([]byte("Hello World!"))
}

func main() {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/env/", envHandler)
	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal("Unable to listen on port 5000 : ", err)
	}
}
