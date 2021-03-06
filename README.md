![Build Status](https://travis-ci.org/JamesDunne/http.svg?branch=master)

# What it is
A CLI tool for invoking modern HTTP APIs, making use of temp files for sharing HTTP headers across multple requests.

# Installation

Download the latest released binaries from [GitHub releases](https://github.com/JamesDunne/http/releases)!

Or you can easily build from source.

```
$ go get github.com/JamesDunne/http
$ go build
or
$ go install
```

Go tools are required for installation. Resulting binary is named `http`.

# Usage

```
$ http
Usage:
http <command or HTTP method> [args...]

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
```

In the above usage statements, `[name]` means an optional argument, and `<name>` means a required argument.

This commandline interface is subject to change. I'll likely keep the same command names, but argument parsing might grow to include some useful sub-features. Command names are not case-sensitive.

# Examples

As an example, let's target the Travis CI build system's HTTP API.

First, let's set up a base URL to use for all subsequent requests. This is a required first step before invoking any HTTP APIs.

```
$ http url https://api.travis-ci.com/
```

Let's confirm that base URL is set:
```
$ http url
https://api.travis-ci.com/
```

The environment is now set up such that all subsequent requests are made to URLs that begin with this absolute URL.

Now let's set up some common headers that Travis requires:

```
$ http set Accepts application/vnd.travis-ci.2+json
$ http set User-Agent http/0.1
```

Travis requires these two headers at the bare minimum for all requests, regardless of authorization. User-Agent can be whatever you want.

In order to do anything remotely interesting with Travis's API, we need to authorize ourselves to their API to prove we have an account there.

Travis is tightly integrated with GitHub and even delegates to GitHub for authorization. According to Travis's docs, we need to acquire a GitHub personal access token (via GitHub's website) and convert that into a Travis authorization token using Travis's API.

```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | http post auth/github application/json
```

As you can see, an `echo` statement appears first because we need to send the POST body data via `stdin` to `http`. We do this by piping (`|`) the `stdout` of the `echo` process to the `stdin` of the `http` process.

We use the `http`'s `post` command which sends an HTTP POST request to the API server.

The first argument is the relative URL `auth/github` to POST the request to; this is relative to the absolute base URL we supplied earlier and set in the environment, composing to the final URL `https://api.travis-ci.com/auth/github`.

Last is the optional `content-type` argument which is explcitly set to `application/json` here, but `application/json` is the default `content-type` anyway so it's a bit redundant in this example.

Let's see what Travis API responds with:

```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | http post auth/github application/json
POST https://api.travis-ci.com/auth/github
User-Agent: http/0.1
Accepts: application/vnd.travis-ci.2+json
Content-Type: application/json

{"github_token":"really-long-hex-string-here-REDACTED"}

Sending HTTP request...

StatusCode: 200
{"access_token":"access-token-REDACTED"}
```

That "access_token" JSON data at the bottom is the API response body containing the token we need to set for future requests. The lines above it are the request headers and POST body that was sent.

Only the actual HTTP response body is written to `stdout`, all else is written to `stderr` which makes it easy to redirect/ignore whichever part you find uninteresting.

As an example, let's ignore all the `stderr` output:
```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | . http post auth/github 2>/dev/null
{"access_token":"access-token-REDACTED"}$ _cursor here_
```

As a matter of priniciple, `http` never outputs anything (not even extra trailing newlines) to `stdout` that did not come directly from the HTTP response body. That's why the shell '$' sigil appears on the same line as the end of the JSON content; there was no extra newline outputted.

Back to our API example...

Now that we have an access token from Travis, we should set that into the "Authorization" header that Travis requires for authorized requests:
```
$ http set Authorization "token \"access-token-REDACTED\""
```

Note that Travis requires literal double-quotes surrounding the access token. THAT is the gateway to bash string quoting hell. Be warned.

What do we have so far in our HTTP headers? I forget...
```
$ http list
Authorization: token "access-token-REDACTED"
Accepts: application/vnd.travis-ci.2+json
User-Agent: http/0.1
```

Oh, right. Good to know.

Now that we've got an authorization token, let's check out some authorized APIs, namely the `cache` entities:
```
$ http get repos/redacted-org-name/redacted-repo-name/caches?branch=master
GET https://api.travis-ci.com/redacted-org-name/redacted-repo-name/caches?branch=master
Accepts: application/vnd.travis-ci.2+json
User-Agent: http/0.1
Authorization: token "access-token-REDACTED"

Sending HTTP request...

StatusCode: 200
{"caches":[{"repository_id":0,"size":207755213,"slug":"cache--python-2.7","branch":"master","last_modified":"2015-02-27T18:56:19Z"}]}
```

Cool. Let's delete that cache:
```
$ http delete repos/redacted-org-name/redacted-repo-name/caches?branch=master
(redundant output; same as GET)
```

Simple!

Now how about clearing all HTTP headers?
```
$ http clear
```

Note that `clear` only clears HTTP headers, not the base URL. To clear the entire environment, use `reset`.

# Justification

I started this project as the ultimate result of a bit of yak-shaving. I was researching how to clear out [Travis CI](http://travis-ci.org/)'s caches and came across their HTTP API for doing so. My first instinct was to attempt HTTP API calls from the OS X commandline using `curl`. Of course, after a bit of testing, it seems standard `curl` on OS X does not have HTTPS enabled. Fail.

Next up was `wget`, which does work and supports HTTPS out of the box. I tried to wrap up all the wget boilerplate into a simple bash script, but failed because bash scripting is completely unreasonable and insane. Bash makes basic string manipulation and quoting so incredibly difficult and obtuse that I just gave up. I gave it several honest tries and nothing was working; the bash man page was no help either.

Even if I did manage to wrap wget in a script, its output is not flexible enough for programmatic control (for my tastes). It seems more fit for single-use requests made by humans than multi-use requests made by programs.

I decided I can do a better job writing a small tool in Go that only does HTTP API invocations in an extremely simple and direct way, consistent with how modern HTTP APIs are implemented and what they expect. Making the common thing easy to do is critical in reducing overall friction in any process.
