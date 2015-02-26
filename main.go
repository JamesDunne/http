package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func do_http(http_method string, body_required bool, args []string) {
	// Validate environment variables first:
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
		os.Exit(2)
		return
	}

	// Get extra headers:
	headers_s := os.Getenv("HTTPCLI_HEADERS")
	headers := make(map[string]string)
	if headers_s != "" {
		err := json.Unmarshal([]byte(headers_s), &headers)
		if err != nil {
			Error("Error parsing JSON from $HTTPCLI_HEADERS: %s\n", err)
			os.Exit(2)
			return
		}
	}

	if len(args) < 1 {
		Error("Missing required relative URL\n")
		os.Exit(1)
		return
	}

	// Parse relative URL:
	rel_url_s := args[0]
	rel_url, err := url.Parse(rel_url_s)
	if err != nil {
		Error("Error parsing relative URL: %s\n", err)
		os.Exit(1)
		return
	}

	// Combine absolute URL base with relative URL argument:
	api_url := &url.URL{
		Scheme:   base_url.Scheme,
		Host:     base_url.Host,
		User:     base_url.User,
		Path:     path.Join(base_url.Path, rel_url.Path),
		RawQuery: rel_url.RawQuery,
		Fragment: rel_url.Fragment,
	}

	// Set up the request:
	req := &http.Request{
		URL:    api_url,
		Method: http_method,
		Header: make(http.Header),
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Exclude named headers:
	// TODO: parse args for this!
	exclude_headers_arg := ""
	if exclude_headers_arg != "" {
		// Remove excluded headers:
		exclude_headers := strings.Split(exclude_headers_arg, ",")
		for _, name := range exclude_headers {
			delete(req.Header, name)
		}
		fmt.Printf("%s\n", headers)
	}

	// Set up body content-type and data:
	if body_required {
		// Read all of stdin to a `[]byte` buffer:
		body_data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			Error("Error reading stdin: %s", err)
			os.Exit(3)
			return
		}

		// for debugging:
		Error("BODY: %s\n", body_data)

		// Create a Buffer to read the `[]byte` as the HTTP body:
		buf := bytes.NewBuffer(body_data)
		req.Body = ioutil.NopCloser(buf)
		req.ContentLength = int64(buf.Len())
	}

	// Make the request:
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Error("HTTP error: %s\n", err)
		os.Exit(3)
		return
	}

	if resp.StatusCode != 200 {
		Error("%d", resp.StatusCode)
	}
	if resp.Body != nil {
		io.Copy(os.Stdout, resp.Body)
	}
}

func main() {
	args := os.Args[1:][:]
	if len(args) == 0 {
		Error(`Usage:
http-cli <command> [args...]

Commands:
  url    <absolute_url>
    Set base URL in environment.

  header-clear
    Clears all HTTP headers in environment.

  header-set <header_name> <header_value>
    Sets a custom HTTP header in environment.

  GET    <relative-url>
  DELETE <relative-url>
    Invoke HTTP GET or DELETE. No body data is sent.

  POST   <relative_url>
  PUT    <relative_url>
    Invoke HTTP POST or PUT. Body data is read from stdin.

Environment variables:
  * HTTPCLI_URL     = base absolute URL for HTTP requests
  * HTTPCLI_HEADERS = JSON encoding of HTTP headers to pass, e.g.
    {
      "Accepts": "content-type-here"
    }
`)
		os.Exit(1)
		return
	}

	// Determine what to do:
	cmd := args[0]
	body_required := true
	switch strings.ToLower(cmd) {
	case "get", "delete":
		body_required = false
		fallthrough
	case "post", "put":
		do_http(strings.ToUpper(cmd), body_required, args[1:])
		break

	case "url":
		break
	case "header-set":
		break
	case "header-clear":
		break

	default:
		os.Exit(1)
		break
	}

}
