package fakegoogle

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Gateway mounts fake under prefix and proxies everything else to upstream
// (or answers 404 when upstream is nil).
func Gateway(prefix string, fake http.Handler, upstream *url.URL) http.Handler {
	mux := http.NewServeMux()
	if prefix == "" {
		mux.Handle("/", fake)
		return mux
	}
	mux.Handle(prefix+"/", http.StripPrefix(prefix, fake))
	if upstream != nil {
		mux.Handle("/", &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(upstream)
				r.SetXForwarded() // includes X-Forwarded-Host with the public host
			},
		})
	}
	return mux
}
