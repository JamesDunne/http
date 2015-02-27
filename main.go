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

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

var initial_environ, current_environ map[string]string

const env_prefix = "HTTPCLI_"
const header_prefix = env_prefix + "HEADER_"

func headerkey_to_envkey(k string) string {
	return header_prefix + strings.Replace(k, "-", "_", -1)
}

func envkey_to_headerkey(k string) string {
	if !strings.HasPrefix(k, header_prefix) {
		return ""
	}

	return strings.Replace(k[len(header_prefix):], "_", "-", -1)
}

// Convert a []string environment from os.Environ() to a more usable map.
func environ_to_map(environ []string) (m map[string]string) {
	m = make(map[string]string)
	for _, kv := range environ {
		// Ignore things we don't care about:
		if !strings.HasPrefix(kv, env_prefix) {
			continue
		}
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
	err = os.Setenv(key, value)
	if err != nil {
		return err
	}

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
	url_s := current_environ[env_prefix+"URL"]
	if url_s == "" {
		Error("Missing $%sURL env var\n", env_prefix)
		os.Exit(2)
		return nil
	}
	base_url, err := url.Parse(url_s)
	if err != nil {
		Error("Error parsing $%sURL: %s\n", env_prefix, err)
		os.Exit(2)
		return nil
	}
	if !base_url.IsAbs() {
		Error("$%sURL must be an absolute URL\n", env_prefix)
		os.Exit(2)
		return nil
	}
	return base_url
}

func set_abs_url(base_url *url.URL) {
	err := setenv(env_prefix+"URL", base_url.String())
	if err != nil {
		Error("Error setting $%sURL: %s\n", env_prefix, err)
		os.Exit(2)
		return
	}
}

// Output a bash script to modify the environment:
func output_bash_env() {
	// Unset removed headers first:
	for key, _ := range initial_environ {
		if _, ok := current_environ[key]; !ok {
			fmt.Printf("unset %s\n", key)
		}
	}

	// Set new or existing headers:
	for key, value := range current_environ {
		if initial_environ[key] == value {
			continue
		}

		// Bash-escape the value as a single-quoted string with escaping rules:
		bash_escape_value := strings.Replace(value, "\\", "\\\\", -1)
		bash_escape_value = strings.Replace(bash_escape_value, "'", "\\'", -1)

		fmt.Printf("export %s=$'%s'\n", key, bash_escape_value)
	}

	// Comments cannot come first if using `eval`.
	// fmt.Println()
	// fmt.Println("# This output must be `eval`ed in bash in order to have effect on the current environment.")
	// fmt.Println("# Example: $ eval `http ...`")
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
	if len(args) == 0 {
		Error(`Usage:
%s <command> [args...]

Commands:
  url    [absolute_url]
    Get or set base URL in environment.

  -- Managing HTTP headers:
  clear
    Clears all HTTP headers in environment.

  set    <header_name> <header_value>
    Sets a custom HTTP header in environment.

  list
    List current HTTP headers in environment.

  env
    Generate a bash script to export current environment.

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
`, os.Args[0])
		os.Exit(1)
		return
	}

	// Determine what to do:
	cmd := args[0]
	args = args[1:]

	// Copy current environment:
	initial_environ = environ_to_map(os.Environ())
	current_environ = environ_to_map(os.Environ())

	body_required := true
	switch strings.ToLower(cmd) {
	case "get", "delete":
		body_required = false
		fallthrough
	case "post", "put":
		do_http(strings.ToUpper(cmd), body_required, args)
		break

	case "url":
		// Must be evaluated on the bash console as "eval `http header-set ...`"
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
		// Must be evaluated on the bash console as "eval `http header-set ...`"

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
		// Must be evaluated on the bash console as "eval `http header-clear ...`"
		set_headers(nil)

		// Output the bash evaluation statements:
		output_bash_env()
		break

	case "env":
		// Output the bash evaluation statements:
		initial_environ = make(map[string]string)
		output_bash_env()
		break

	default:
		os.Exit(1)
		break
	}

}
