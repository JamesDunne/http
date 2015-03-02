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
		// Trim leading spaces:
		b = strings.TrimLeft(spl[1], " ")
	}
	return
}

// Returns HTTP status code
func do_http(http_method string, args []string) int {
	// Overridden via -q flag:
	quiet_mode := false
	pretty_print := false

	// Determine if body is required based on method:
	body_required := false
	switch http_method {
	case "POST", "PUT":
		body_required = true
		break
	}

	// Parse arguments:
	exclude_headers_arg := ""

	q := args
	xargs := make([]string, 0, len(args))
	for len(q) > 0 {
		arg := q[0]
		if len(arg) >= 2 && arg[0] == '-' {
			// Is arg a switch?
			switch arg[1] {
			case 'x':
				// Exclude headers; comma-delimited:
				q = q[1:]
				if len(q) == 0 {
					Error("Expected comma-delimited list of header names following %s flag\n", arg)
					return -1
				}

				exclude_headers_arg = q[0]
				q = q[1:]
				break
			case 'q':
				// Quiet mode:
				q = q[1:]
				quiet_mode = true
				break
			case 'p':
				// Pretty-print JSON:
				q = q[1:]
				pretty_print = true
				break
			default:
				Error("Unrecognized flag: %s\n", arg)
				return -1
			}
		} else {
			xargs = append(xargs, arg)
			q = q[1:]
		}
	}

	args = xargs
	//Error("%s, %d\n", args, len(args))

	// Get environment:
	base_url := get_base_url()
	headers, _ := get_headers()

	if len(args) == 0 {
		Error("Missing required URL\n")
		return -1
	}

	// Parse relative URL:
	arg_url_s := args[0]
	arg_url, err := url.Parse(arg_url_s)
	if err != nil {
		Error("Error parsing URL: %s\n", err)
		return -1
	}

	// Build a request URL:
	var api_url *url.URL
	if arg_url.IsAbs() {
		// Argument provided an absolute URL; ignore base URL from environment.
		api_url = arg_url
	} else /* if !arg_url.IsAbs() */ {
		// Argument provided a relative URL; require absolute base URL:
		if base_url == nil {
			Error(`Relative URL passed as argument but missing an absolute base URL from
environment. Either supply an absolute URL or use the "http url <base-url>"
command to set an absolute base URL.
`)
			return -1
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
			return -2
		}

		// Default to `application/json` content-type; override with 2nd arg:
		content_type := req.Header.Get("Content-Type")
		if content_type == "" {
			content_type = "application/json"
		}
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

	if !quiet_mode {
		Error("%s %s\n", http_method, api_url)
		req.Header.Write(os.Stderr)
		Error("\n")
		if body_required {
			Error("%s\n\n", body_data)
		}
	}

	// Make the request:
	if !quiet_mode {
		Error("--------------------------------------------------------------------------------\n")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Error("HTTP error: %s\n", err)
		return -2
	}

	// Dump response headers to stderr:
	if !quiet_mode {
		Error("%s\n\n", resp.Status)
		resp.Header.Write(os.Stderr)
	}

	if resp.Body != nil {
		if !quiet_mode {
			Error("\n")
		}

		// Check response content-type:
		content_type := resp.Header.Get("Content-Type")
		content_type, _ = split2(content_type, ";")
		if pretty_print && (content_type == "application/json") {
			// Pretty-print JSON output:

			// ... or use `json.Indent(dst, src, "", "  ")`

			// Decode JSON from response body:
			body_json := make(map[string]interface{})
			dec := json.NewDecoder(resp.Body)
			err := dec.Decode(&body_json)
			if err != nil {
				Error("WARNING: Error decoding JSON response: %s\n", err)
				goto raw_out
			}

			// Pretty-print json:
			out, err := json.MarshalIndent(body_json, "", "  ")
			if err != nil {
				Error("WARNING: Error re-encoding JSON response: %s\n", err)
				goto raw_out
			}

			_, err = io.Copy(os.Stdout, bytes.NewReader(out))
			if err != nil {
				Error("Error copying response body to stdout: %s\n", err)
				// We've likely already written to stdout so we can't redirect.
				return -3
			}

			// Pretty-printing anyway, so output a trailing newline:
			fmt.Println()

			return resp.StatusCode
		}

	raw_out:
		// Copy response body straight to stdout:
		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			Error("Error copying response body to stdout: %s\n", err)
			return resp.StatusCode
		}

		if !quiet_mode {
			// For nicer shell output in the event that stderr -> stdout.
			// We don't want to append any unnecessary \n to stdout.
			Error("\n")
		}
	}

	return resp.StatusCode
}
