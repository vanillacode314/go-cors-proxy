package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "*")
		w.WriteHeader(200)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing url parameter", 400)
		return
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), 500)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirectLocation := req.URL.String()
			proxyURL := "/?url=" + url.QueryEscape(redirectLocation)
			w.Header().Set("Location", proxyURL)
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if err, ok := err.(*url.Error); ok && err.Err == http.ErrUseLastResponse {
		} else {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	defer resp.Body.Close()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")

	for name, values := range resp.Header {
		if name == "Access-Control-Allow-Origin" || name == "Access-Control-Allow-Methods" || name == "Access-Control-Allow-Headers" || name == "Access-Control-Expose-Headers" || name == "Location" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/proxy", proxyHandler)
	log.Printf("CORS proxy server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
