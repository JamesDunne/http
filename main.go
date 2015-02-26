package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
)

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func main() {
	if len(os.Args) <= 2 {
		Error("http-cli <http method> <relative URL>\n")
		os.Exit(1)
		return
	}

	url_s := os.Getenv("HTTPCLI_URL")
	if url_s == "" {
		Error("Missing $HTTPCLI_URL env var\n")
		os.Exit(2)
		return
	}
	base_url, err := url.Parse(url_s)
	if err != nil {
		Error("Error parsing $HTTPCLI_URL: %s\n", err)
		os.Exit(2)
		return
	}
	if !base_url.IsAbs() {
		Error("$HTTPCLI_URL must be an absolute URL\n")
	}

	http_method := os.Args[1]
	rel_url_s := os.Args[2]

	rel_url, err := url.Parse(rel_url_s)
	if err != nil {
		Error("Error parsing relative URL: %s\n", err)
		os.Exit(1)
		return
	}

	api_url := &url.URL{
		Scheme:   base_url.Scheme,
		Host:     base_url.Host,
		User:     base_url.User,
		Path:     path.Join(base_url.Path, rel_url.Path),
		RawQuery: rel_url.RawQuery,
		Fragment: rel_url.Fragment,
	}

	// TODO(jsd): Dumb reparsing of URL here; use `&http.Request{}` initializer and `http.DefaultClient.Do(req)`.
	_ = http_method
	resp, err := http.Get(api_url.String())
	if err != nil {
		Error("HTTP error: %s\n", err)
		os.Exit(3)
		return
	}

	Error("%d", resp.StatusCode)
	if resp.Body != nil {
		io.Copy(os.Stdout, resp.Body)
	}
}
