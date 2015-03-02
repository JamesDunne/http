package main

import (
	"bytes"
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

var env_path = path.Join(TempDir(), ".http-cli.env")

func load_env() {
	// Load current environment:
	var env_data []byte
	env_data, err := ioutil.ReadFile(env_path)
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

	err := ioutil.WriteFile(env_path, []byte(env), 0600)
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

func get_abs_url() *url.URL {
	url_s := current_environ["URL"]
	if url_s == "" {
		Error("No base URL set\n")
		os.Exit(2)
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

func set_abs_url(base_url *url.URL) {
	new_value := ""
	if base_url != nil {
		if !base_url.IsAbs() {
			Error("Base URL must be an absolute URL\n")
			os.Exit(2)
			return
		}
		new_value = base_url.String()
	}

	err := setenv("URL", new_value)
	if err != nil {
		Error("Error setting base URL: %s\n", err)
		os.Exit(2)
		return
	}
}

func do_http(http_method string, body_required bool, args []string) {
	// Get environment:
	base_url := get_abs_url()
	headers := get_headers()

	if len(args) == 0 {
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

	// Prevent base path from being empty:
	base_path := base_url.Path
	if base_path == "" {
		base_path = "/"
	}

	// Combine absolute URL base with relative URL argument:
	api_url := &url.URL{
		Scheme:   base_url.Scheme,
		Host:     base_url.Host,
		User:     base_url.User,
		Path:     path.Join(base_path, rel_url.Path),
		RawQuery: rel_url.RawQuery,
		Fragment: rel_url.Fragment,
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
	for key, values := range req.Header {
		Error("%s: %s\n", key, strings.Join(values, " "))
	}
	Error("\n")
	if body_required {
		Error("%s\n\n", body_data)
	}

	// Make the request:
	Error("Sending HTTP request...\n\n")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Error("HTTP error: %s\n", err)
		os.Exit(3)
		return
	}

	Error("StatusCode: %d\n", resp.StatusCode)
	if resp.Body != nil {
		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			Error("Error copying response body to stdout: %s\n", err)
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
  url    [absolute_url]
    Gets or sets base URL in environment.

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
  GET    <relative-url>
  DELETE <relative-url>
    Invoke HTTP GET or DELETE.
	<relative-url> is combined with [absolute_url] from environment.
	No body data is sent.

  POST   <relative_url> [content-type]
  PUT    <relative_url> [content-type]
    Invoke HTTP POST or PUT. Body data is read from stdin and buffered.
	[content-type] default is "application/json".
`, tool_name)
		os.Exit(1)
		return
	}

	// Determine what to do:
	cmd := args[0]
	args = args[1:]

	load_env()

	body_required := true
	switch strings.ToLower(cmd) {
	case "get", "delete":
		body_required = false
		fallthrough
	case "post", "put":
		do_http(strings.ToUpper(cmd), body_required, args)
		break

	case "url":
		if len(args) == 0 {
			base_url := get_abs_url()
			fmt.Printf("%s", base_url)
			fmt.Fprintln(os.Stderr)
		} else if len(args) == 1 {
			base_url, err := url.Parse(args[0])
			if err != nil {
				Error("Error parsing absolute URL: %s\n", err)
				os.Exit(1)
				return
			}
			set_abs_url(base_url)
			store_env()
		}
		break

	case "reset":
		set_headers(nil)
		set_abs_url(nil)
		store_env()
		break

	case "list":
		// Get HTTP headers from environment:
		headers := get_headers()
		for key, values := range headers {
			fmt.Printf("%s: %s\n", key, strings.Join(values, " "))
		}
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

		}

		set_headers(headers)
		store_env()
		break

	case "clear":
		set_headers(nil)
		store_env()
		break

	default:
		os.Exit(1)
		break
	}

}
