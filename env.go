package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

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
