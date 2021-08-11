package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/tomnomnom/linkheader"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type Director func(*http.Request)

func (f Director) Then(g Director) Director {
	return func(req *http.Request) {
		f(req)
		g(req)
	}
}

func hostDirector(host string) Director {
	return func(req *http.Request) {
		req.Host = host
	}
}

func UpdateResponse(r *http.Response) error {

	fmt.Println(r.StatusCode)
	fmt.Println(r.Header["Content-Type"])

	if r.StatusCode != http.StatusOK {
		return nil
	}

	defer r.Body.Close()

	var reader io.ReadCloser
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		fmt.Println("is gzip")
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			return err
		}
		defer reader.Close()
	default:
		reader = r.Body
	}

	responsePayload := []interface{}{}
	err := json.NewDecoder(reader).Decode(&responsePayload)
	if err != nil {
		return err
	}

	newResponseObject := map[string]interface{}{
		"results": responsePayload,
	}

	for _, link := range linkheader.Parse(r.Header.Get("Link")) {
		newResponseObject["link-header-rel-"+link.Rel] = link.URL
	}

	newResponseObject["link-header-raw-value"] = r.Header.Get("Link")

	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(newResponseObject)
	if err != nil {
		return err
	}

	r.Body = ioutil.NopCloser(buf)
	r.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}
	return nil
}

func main() {
	url, _ := url.Parse("https://api.getguru.com")
	proxy := httputil.NewSingleHostReverseProxy(url)

	d := proxy.Director
	// sequence the default director with our host director
	proxy.Director = Director(d).Then(hostDirector(url.Hostname()))
	proxy.ModifyResponse = UpdateResponse

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		// Turn off GZIP/compression, couldn't get the decoding working in UpdateResponse()
		r.Header.Set("Accept-Encoding", "")
		proxy.ServeHTTP(rw, r)
	})

	port := "9090"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
