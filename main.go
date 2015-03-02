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

func main() {
	args := os.Args[1:][:]
	_, tool_name := filepath.Split(os.Args[0])
	if len(args) == 0 {
		Error(`Usage:
%s <command or HTTP method> [args...]

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

HTTP:
  GET     <url>
  *       <url>
    Invoke HTTP method against <url>; if <url> is relative, <url> is combined with <base_url>.
	No body data is sent for these HTTP methods.

  POST    <url> [content-type]
  PUT     <url> [content-type]
  *       <url> <content-type>
    Invoke HTTP method against <url>; if <url> is relative, <url> is combined with <base_url>

    Request body data is read from stdin (buffered) and submitted with Content-Length.
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
