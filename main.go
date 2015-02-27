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
	"strings"
)

var initial_environ, current_environ map[string]string

// Convert a []string environment from os.Environ() to a more usable map.
func environ_to_map(environ []string) (m map[string]string) {
	m = make(map[string]string)
	for _, kv := range environ {
		if !strings.HasPrefix(kv, "HTTPCLI_") {
			continue
		}
		equ := strings.IndexByte(kv, '=')
		if equ == -1 {
			continue
		}
		name, value := kv[:equ], kv[equ+1:]
		m[name] = value
	}
	return
}

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func get_headers() (headers http.Header) {
	// Get HTTP headers from environment:
	headers = make(http.Header)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HTTPCLI_") {
			continue
		}
		h := kv[len("HTTPCLI_"):]
		equ := strings.IndexByte(h, '=')
		if equ == -1 {
			continue
		}
		name, value := h[:equ], h[equ+1:]
		headers.Set(name, value)
	}
	return
}

func set_headers(headers http.Header) {
	for key, values := range headers {
		err := os.Setenv("HTTPCLI_"+key, strings.Join(values, " "))
		if err != nil {
			Error("Error setting $HTTPCLI_HEADERS: %s\n", err)
			os.Exit(2)
			return
		}
	}
}

func get_abs_url() *url.URL {
	url_s := os.Getenv("HTTPCLI_URL")
	if url_s == "" {
		Error("Missing $HTTPCLI_URL env var\n")
		os.Exit(2)
		return nil
	}
	base_url, err := url.Parse(url_s)
	if err != nil {
		Error("Error parsing $HTTPCLI_URL: %s\n", err)
		os.Exit(2)
		return nil
	}
	if !base_url.IsAbs() {
		Error("$HTTPCLI_URL must be an absolute URL\n")
		os.Exit(2)
		return nil
	}
	return base_url
}

func set_abs_url(base_url *url.URL) {
	err := os.Setenv("HTTPCLI_URL", base_url.String())
	if err != nil {
		Error("Error setting $HTTPCLI_URL: %s\n", err)
		os.Exit(2)
		return
	}
}

func output_bash_env() {
	// Unset removed headers first:

	curr_environ := os.Environ()

	// Output the bash script to modify the environment:
	for _, key := range curr_environ {
		value := os.Getenv(key)
		// Bash-escape the value as a single-quoted string with escaping rules:
		bash_escape_value := strings.Replace(value, "\\", "\\\\", -1)
		bash_escape_value = strings.Replace(bash_escape_value, "'", "\\'", -1)
		fmt.Printf("export %s=$'%s'\n", key, bash_escape_value)
	}
	fmt.Println()
	fmt.Println("# This output must be `eval`ed in bash in order to have effect on the current environment.")
	fmt.Println("# Example: $ eval `http-cli ...`")
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

		if len(args) >= 2 {
			req.Header.Set("Content-Type", args[1])
		}

		// Create a Buffer to read the `[]byte` as the HTTP body:
		buf := bytes.NewBuffer(body_data)
		req.Body = ioutil.NopCloser(buf)
		req.ContentLength = int64(buf.Len())
	}

	// for debugging:
	Error("%s %s\n", http_method, api_url)
	for key, values := range req.Header {
		Error("%s: %s\n", key, strings.Join(values, " "))
	}
	Error("\n%s\n", body_data)

	// Make the request:
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Error("HTTP error: %s\n", err)
		os.Exit(3)
		return
	}

	if resp.StatusCode != 200 {
		Error("%d\n", resp.StatusCode)
	}
	if resp.Body != nil {
		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			Error("Error copying: %s\n", err)
		}
		fmt.Println()
	}
}

func main() {
	args := os.Args[1:][:]
	if len(args) == 0 {
		Error(`Usage:
http-cli <command> [args...]

Commands:
  url    [absolute_url]
    Get or set base URL in environment.

  header-clear
    Clears all HTTP headers in environment.

  header-set <header_name> <header_value>
    Sets a custom HTTP header in environment.

  header-list
    List current HTTP headers in environment.

  GET    <relative-url>
  DELETE <relative-url>
    Invoke HTTP GET or DELETE. No body data is sent.

  POST   <relative_url> [content-type]
  PUT    <relative_url> [content-type]
    Invoke HTTP POST or PUT. Body data is read from stdin.
`)
		os.Exit(1)
		return
	}

	// Determine what to do:
	cmd := args[0]
	args = args[1:]

	// Copy current environment:
	initial_environ = environ_to_map(os.Environ())

	body_required := true
	switch strings.ToLower(cmd) {
	case "get", "delete":
		body_required = false
		fallthrough
	case "post", "put":
		do_http(strings.ToUpper(cmd), body_required, args)
		break

	case "url":
		// Must be evaluated on the bash console as "eval `http-cli header-set ...`"
		if len(args) == 0 {
			base_url := get_abs_url()
			fmt.Printf("%s\n", base_url)
		} else if len(args) == 1 {
			base_url, err := url.Parse(args[0])
			if err != nil {
				Error("Error parsing absolute URL: %s\n", err)
				os.Exit(1)
				return
			}
			set_abs_url(base_url)
			output_bash_env()
		}
		break

	case "list":
		// Get HTTP headers from environment:
		headers := get_headers()
		for key, values := range headers {
			fmt.Printf("%s: %s\n", key, strings.Join(values, " "))
		}
		break

	case "set":
		// Must be evaluated on the bash console as "eval `http-cli header-set ...`"

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

		// Output the bash evaluation statements:
		output_bash_env()
		break

	case "clear":
		// Must be evaluated on the bash console as "eval `http-cli header-clear ...`"
		set_headers(nil)

		// Output the bash evaluation statements:
		output_bash_env()
		break

	default:
		os.Exit(1)
		break
	}

}
