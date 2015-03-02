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
	"path/filepath"
	"strings"
)

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

var initial_environ, current_environ map[string]string

const header_prefix = "HEADER_"

func load_env() {
	// Load current environment:
	var env_data []byte
	env_data, err := ioutil.ReadFile(env_path())
	if err != nil {
		env_data = []byte{}
	}
	lines := strings.Split(string(env_data), "\n")

	// Copy current environment:
	initial_environ = environ_to_map(lines)
	current_environ = environ_to_map(lines)
}

func store_env() {
	// Combine all env vars into a string:
	env := ""
	for key, value := range current_environ {
		env += key + "=" + value + "\n"
	}

	err := ioutil.WriteFile(env_path(), []byte(env), 0600)
	if err != nil {
		panic(err)
	}
}

func headerkey_to_envkey(k string) string {
	return header_prefix + strings.Replace(k, "-", "_", -1)
}

func envkey_to_headerkey(k string) string {
	if !strings.HasPrefix(k, header_prefix) {
		return ""
	}

	return strings.Replace(k[len(header_prefix):], "_", "-", -1)
}

// Convert a []string environment to a more usable map.
func environ_to_map(environ []string) (m map[string]string) {
	m = make(map[string]string)
	for _, kv := range environ {
		// Split by first '=':
		equ := strings.IndexByte(kv, '=')
		if equ == -1 {
			continue
		}
		name, value := kv[:equ], kv[equ+1:]
		m[name] = value
	}
	return
}

func setenv(key, value string) (err error) {
	// Replicate the setting in the current_environ map:
	if value == "" {
		delete(current_environ, key)
		return nil
	}

	current_environ[key] = value
	return nil
}

// Get HTTP headers from environment:
func get_headers() (headers http.Header) {
	headers = make(http.Header)
	for key, value := range current_environ {
		if !strings.HasPrefix(key, header_prefix) {
			continue
		}
		name := envkey_to_headerkey(key)
		headers.Set(name, value)
	}
	return
}

// Set HTTP headers back to environment:
func set_headers(headers http.Header) {
	// Unset removed headers first:
	for key, _ := range current_environ {
		if !strings.HasPrefix(key, header_prefix) {
			continue
		}
		name := envkey_to_headerkey(key)
		if _, ok := headers[name]; !ok {
			setenv(key, "")
		}
	}

	// Set new headers:
	for key, values := range headers {
		name := headerkey_to_envkey(key)
		value := strings.Join(values, " ")
		err := setenv(name, value)
		if err != nil {
			Error("Error setting $%s: %s\n", name, err)
			os.Exit(2)
			return
		}
	}
}

func get_base_url() *url.URL {
	url_s := current_environ["URL"]
	if url_s == "" {
		return nil
	}
	base_url, err := url.Parse(url_s)
	if err != nil {
		Error("Error parsing base URL: %s\n", err)
		os.Exit(2)
		return nil
	}
	if !base_url.IsAbs() {
		Error("Base URL must be an absolute URL\n")
		os.Exit(2)
		return nil
	}
	return base_url
}

func set_base_url(url_s string) {
	new_value := ""
	if url_s != "-" {
		base_url, err := url.Parse(url_s)
		if err != nil {
			Error("Error parsing absolute URL: %s\n", err)
			os.Exit(1)
			return
		}

		if base_url != nil {
			if !base_url.IsAbs() {
				Error("Base URL must be an absolute URL\n")
				os.Exit(2)
				return
			}
			new_value = base_url.String()
		}
	}
	err := setenv("URL", new_value)
	if err != nil {
		Error("Error setting base URL: %s\n", err)
		os.Exit(2)
		return
	}
}

func Split2(s string, sep string) (a string, b string) {
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
		api_url := &url.URL{
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
		content_type, _ = Split2(content_type, ";")
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

func main() {
	args := os.Args[1:][:]
	_, tool_name := filepath.Split(os.Args[0])
	if len(args) == 0 {
		Error(`Usage:
%s <command> [args...]

Commands:
  url
    Gets current base URL from environment.
  url    <base_url>
    Sets base URL in environment; must be absolute URL.

  reset
    Resets environment; clears out HTTP headers and base URL.

  -- Managing HTTP headers:
  set    <header_name> <header_value>
    Sets a custom HTTP header in environment.

  list
    List current HTTP headers in environment.

  clear
    Clears all HTTP headers in environment.

  -- Making HTTP requests:
  GET     <url>
  DELETE  <url>
  *       <url>
    Invoke HTTP method against <url>; if <url> is relative, <url> is combined with <base_url>.
	No body data is sent for these HTTP methods.

  POST    <url> [content-type]
  PUT     <url> [content-type]
  *       <url> <content-type>
    Invoke HTTP method against <url>; if <url> is relative, <url> is combined with <base_url>

    Request body data is read from stdin and buffered and submitted with Content-Length.
	[content-type] default is "application/json" so can be omitted if default is preferred.
	For anything but POST and PUT, <content-type> is required if the method is non-standard
	and wants to submit a request body.
`, tool_name)
		os.Exit(1)
		return
	}

	// Determine what to do:
	cmd := args[0]
	args = args[1:]

	// Load environment data from file:
	load_env()

	// Process command:
	switch strings.ToLower(cmd) {
	case "url":
		if len(args) == 0 {
			base_url := get_base_url()
			fmt.Printf("%s", base_url)
			fmt.Fprintln(os.Stderr)
		} else if len(args) == 1 {
			set_base_url(args[0])
			store_env()
		}
		break

	case "env":
		base_url_s := ""
		base_url := get_base_url()
		if base_url != nil {
			base_url_s = base_url.String()
		}
		fmt.Printf("%s\n\n", base_url_s)

		// Get HTTP headers from environment:
		get_headers().Write(os.Stdout)
		break

	case "list":
		// Get HTTP headers from environment:
		get_headers().Write(os.Stdout)
		break

	case "set":
		// Get HTTP headers from environment:
		headers := get_headers()
		if len(args) == 2 {
			// Set a new HTTP header:
			headers.Set(args[0], args[1])
		} else if len(args) == 1 {
			delete(headers, args[0])
		} else {
			Error("Missing header name and value\n")
			os.Exit(1)
			return
		}

		set_headers(headers)
		store_env()
		break

	case "clear":
		set_headers(nil)
		store_env()
		break

	case "reset":
		set_headers(nil)
		set_base_url("")
		store_env()
		break

	// HTTP methods:
	default:
		do_http(strings.ToUpper(cmd), args)
		break
	}

}
