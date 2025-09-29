package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
)

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

// setCORS now accepts an http.Header so it can be applied to both
// the OPTIONS response and the proxied response.
func setCORS(h http.Header) {
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "*")
	h.Set("Access-Control-Expose-Headers", "*")
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Handle preflight requests directly
	if r.Method == http.MethodOptions {
		setCORS(w.Header())
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Extract and validate target URL
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

	// 3. Create the Reverse Proxy
	proxy := &httputil.ReverseProxy{
		// Rewrite configures the outgoing request to the target server
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Do NOT use pr.SetURL(parsed) here, as it merges the "?url=" parameter
			// and the "/proxy" path into the upstream request.

			// Instead, completely overwrite the outbound URL with the parsed target URL.
			pr.Out.URL = parsed
			pr.Out.Host = parsed.Host
		},

		// ModifyResponse intercepts the response before it's sent to the client
		ModifyResponse: func(resp *http.Response) error {
			// Handle Redirects: Rewrite the Location header so the client continues
			// to follow the redirect through our proxy, rather than bypassing it.
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				loc := resp.Header.Get("Location")
				if loc != "" {
					// parsed.Parse gracefully handles resolving relative redirect URLs
					locURL, err := parsed.Parse(loc)
					if err == nil {
						proxyURL := "/proxy?url=" + url.QueryEscape(locURL.String())
						resp.Header.Set("Location", proxyURL)
					}
				}
			}

			// Ensure our proxy CORS headers overwrite any upstream CORS headers
			setCORS(resp.Header)
			return nil
		},

		// ErrorHandler gracefully handles issues like connection timeouts
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", targetURL, err)
			http.Error(w, "Proxy request failed", http.StatusBadGateway)
		},
	}

	// 4. Execute the proxy request
	// ReverseProxy automatically uses the request context and efficiently streams the body via io.Copy
	proxy.ServeHTTP(w, r)
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
