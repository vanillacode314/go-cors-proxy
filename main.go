package main

import (
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
)

var client = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func isPrivateOrBlocked(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err == nil {
		return addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() || addr.IsLinkLocalUnicast()
	}
	blocked := []string{
		"localhost",
		"metadata.google.internal",
		"169.254.169.254",
		"metadata.aws",
	}
	for _, b := range blocked {
		if host == b || host == b+".internal" {
			return true
		}
	}
	return false
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		setCORS(w)
		w.WriteHeader(200)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		http.Error(w, "Only http and https schemes are allowed", http.StatusBadRequest)
		return
	}
	if isPrivateOrBlocked(parsed.Hostname()) {
		http.Error(w, "Access to private/blocked hosts is not allowed", http.StatusForbidden)
		return
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok && urlErr.Err == http.ErrUseLastResponse {
			// Redirect blocked - write 301 with proxy URL
			setCORS(w)
			proxyURL := "/?url=" + url.QueryEscape(urlErr.URL)
			w.Header().Set("Location", proxyURL)
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		http.Error(w, "Request failed", http.StatusBadGateway)
		return
	}

	defer resp.Body.Close()

	setCORS(w)

	for name, values := range resp.Header {
		if name == "Access-Control-Allow-Origin" || name == "Access-Control-Allow-Methods" || name == "Access-Control-Allow-Headers" || name == "Access-Control-Expose-Headers" || name == "Location" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	// Stream passthrough with flushing for SSE/LLM token streaming
	// Use small buffer to pass through chunks as they arrive
	flush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flush.Flush()
		}
		if err != nil {
			break
		}
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/proxy", proxyHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Printf("CORS proxy server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
