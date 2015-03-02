package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func print_url_blank() {
	base_url := get_base_url()
	if base_url == nil {
		return
	}
	fmt.Printf("%s\n", base_url.String())
}

func print_url() {
	base_url := get_base_url()
	if base_url != nil {
		tmp := *base_url
		tmp.RawQuery = ""
		fmt.Printf("URL:   %s\n", tmp.String())
		fmt.Printf("Query: %s\n", base_url.RawQuery)
	} else {
		fmt.Printf("No base URL\n")
	}
}

func print_headers() {
	headers, n := get_headers()
	if n == 0 {
		fmt.Printf("No HTTP headers\n")
	} else {
		headers.Write(os.Stdout)
	}
}

func main() {
	args := os.Args[1:][:]
	_, tool_name := filepath.Split(os.Args[0])
	if len(args) == 0 {
		Error(`Usage:
%s <command or HTTP method> [args...]

Commands:
  url <base_url>     - Sets base URL in environment; must be absolute URL. To
                       clear base URL, use "-" as <base_url>.
  url                - Displays current base URL from environment.
  env                - Displays environment: URL, blank line, then HTTP headers
                       (one per line).
  session            - Displays environment session ID. Use $HTTPCLI_SESISON_ID
                       env var to override. Default is "yyyy-MM-dd-########"
                       with datestamp and parent process pid.
  reset              - Resets environment; clears out HTTP headers and base URL.

  set <name> <value> - Sets a custom HTTP header in environment.
  list               - List current HTTP headers in environment.
  clear              - Clears all HTTP headers in environment.

HTTP:
  <method> <url> [content-type]
    Invoke HTTP method against <url>; if <url> is relative, <url> is combined
    with <base_url>.

    If <method> is POST or PUT then a request body is required. [content-type]
    is required if <method> is not POST or PUT but a request body is needed.

    Request body is read from stdin until EOF, buffered into memory, and
    submitted with a calculated Content-Length header value. Alternate
    Transfer-Modes are not supported currently.

    [content-type] default is "application/json"
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
			print_url_blank()
		} else if len(args) == 1 {
			set_base_url(args[0])
			store_env()
		}
		break

	case "env":
		base_url := get_base_url()
		if base_url != nil {
			fmt.Printf("%s\n\n", base_url.String())
		} else {
			fmt.Printf("No base URL\n\n")
		}

		// Get HTTP headers from environment:
		print_headers()
		break

	case "session":
		// Print current session ID for the environment:
		fmt.Printf("%s\n", SessionID())
		break

	case "list":
		// Get HTTP headers from environment:
		print_headers()
		break

	case "set":
		// Get HTTP headers from environment:
		headers, _ := get_headers()
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
