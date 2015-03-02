// http.go
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

func split2(s string, sep string) (a string, b string) {
	spl := strings.SplitN(s, sep, 2)
	a = spl[0]
	if len(spl) > 1 {
		b = spl[1]
	}
	return
}

func do_http(http_method string, args []string) {
	// Determine if body is required based on method:
	body_required := false
	switch http_method {
	case "POST", "PUT":
		body_required = true
		break
	}

	// Parse arguments:

	// Get environment:
	base_url := get_base_url()
	headers := get_headers()

	if len(args) == 0 {
		Error("Missing required URL\n")
		os.Exit(1)
		return
	}

	// Parse relative URL:
	arg_url_s := args[0]
	arg_url, err := url.Parse(arg_url_s)
	if err != nil {
		Error("Error parsing URL: %s\n", err)
		os.Exit(1)
		return
	}

	// Build a request URL:
	var api_url *url.URL
	if arg_url.IsAbs() {
		// Argument provided an absolute URL; ignore base URL from environment.
		api_url = arg_url
	} else /* if !arg_url.IsAbs() */ {
		// Argument provided a relative URL; require absolute base URL:
		if base_url == nil {
			Error("Relative URL passed as argument but missing an absolute base URL from environment. Use `http url <base-url>` command to set one first.\n")
			os.Exit(1)
			return
		}

		// Combine base URL as absolute with arg URL as relative.

		// Prevent base path from being empty:
		base_path := base_url.Path
		if base_path == "" {
			base_path = "/"
		}

		// Treat argument URL as relative
		rel_url := arg_url

		// Combine absolute URL base with relative URL argument:
		api_url = &url.URL{
			Scheme:   base_url.Scheme,
			Host:     base_url.Host,
			User:     base_url.User,
			Path:     path.Join(base_path, rel_url.Path),
			RawQuery: base_url.RawQuery,
			Fragment: rel_url.Fragment,
		}

		// Add rel_url's query to base_url's:
		q := api_url.Query()
		for k, v := range rel_url.Query() {
			q[k] = v
		}
		api_url.RawQuery = q.Encode()
	}

	// Set up the request:
	req := &http.Request{
		URL:    api_url,
		Method: http_method,
		Header: headers,
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
	}

	if len(args) >= 2 {
		body_required = true
	}

	// Set up body content-type and data:
	var body_data []byte
	if body_required {
		// Read all of stdin to a `[]byte` buffer:
		body_data, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			Error("Error reading stdin: %s", err)
			os.Exit(3)
			return
		}

		// Default to `application/json` content-type; override with 2nd arg:
		content_type := "application/json"
		if len(args) >= 2 {
			content_type = args[1]
		}
		req.Header.Set("Content-Type", content_type)

		// Create a Buffer to read the `[]byte` as the HTTP body:
		buf := bytes.NewBuffer(body_data)

		// Set the Body and ContentLength:
		req.ContentLength = int64(buf.Len())
		req.Body = ioutil.NopCloser(buf)
	}

	// for debugging:
	Error("%s %s\n", http_method, api_url)
	req.Header.Write(os.Stderr)
	if body_required {
		Error("%s\n\n", body_data)
	}

	// Make the request:
	Error("--------------------------------------------------------------------------------\n")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Error("HTTP error: %s\n", err)
		os.Exit(3)
		return
	}

	// Dump response headers to stderr:
	Error("%s\n", resp.Status)
	resp.Header.Write(os.Stderr)

	if resp.Body != nil {
		Error("\n")

		content_type := resp.Header.Get("Content-Type")
		content_type, _ = split2(content_type, ";")
		if content_type == "application/json" {
			// Pretty-print JSON output:

			// ... or use `json.Indent(dst, src, "", "  ")`

			// Decode JSON from response body:
			body_json := make(map[string]interface{})
			dec := json.NewDecoder(resp.Body)
			err := dec.Decode(&body_json)
			if err != nil {
				Error("Error decoding JSON response: %s\n", err)
				return
			}

			// Pretty-print json:
			out, err := json.MarshalIndent(body_json, "", "  ")
			if err != nil {
				Error("Error decoding JSON response: %s\n", err)
				return
			}

			_, err = io.Copy(os.Stdout, bytes.NewReader(out))
			if err != nil {
				Error("Error copying response body to stdout: %s\n", err)
				return
			}
		} else {
			// Copy response body straight to stdout:
			_, err = io.Copy(os.Stdout, resp.Body)
			if err != nil {
				Error("Error copying response body to stdout: %s\n", err)
				return
			}
		}

		// For nicer shell output in the event that stderr -> stdout.
		// We don't want to append any unnecessary \n to stdout.
		fmt.Fprintln(os.Stderr)
	}
}
