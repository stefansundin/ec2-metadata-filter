package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func acceptableRequest(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" {
		return false
	}

	if r.Header.Get("Metadata-Flavor") == "Amazon" {
		return true
	}

	whitelistedUserAgentPrefixes := []string{
		"aws-chalice/",
		"aws-cli/",
		"aws-sdk-",
		"Boto3/",
		"Botocore/",
	}

	ua := r.UserAgent()
	if ua == "" {
		// no user-agent header was sent
		return false
	}

	for _, v := range whitelistedUserAgentPrefixes {
		if strings.HasPrefix(ua, v) {
			return true
		}
	}
	return false
}

func main() {
	remote, err := url.Parse("http://169.254.169.254")
	if err != nil {
		panic(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)

	handleRequest := func(w http.ResponseWriter, r *http.Request) {
		if acceptableRequest(r) {
			log.Printf("Proxying request to %s from User-Agent: %s\n", r.URL, r.UserAgent())
			proxy.ServeHTTP(w, r)
		} else {
			log.Printf("Blocked request to %s from User-Agent: %s\n", r.URL, r.UserAgent())
			w.WriteHeader(http.StatusBadRequest) // 400
		}
	}

	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 16925
	}
	log.Printf("Listening on port: %d\n", port)

	http.HandleFunc("/", handleRequest)
	err = http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
