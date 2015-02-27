# What it is
A CLI tool for invoking modern HTTP APIs, using easily-modifiable environment variables for cross-request context.

# Usage

```
$ ./http
Usage:
./http <command> [args...]

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
```

In the above usage statements, `[name]` means an optional argument, and `<name>` means a required argument.

This commandline interface is subject to change. I'll likely keep the same command names, but argument parsing might grow to include some useful sub-features. Command names are not case-sensitive.

# Examples

As an example, let's target the Travis CI build system's HTTP API.

First, let's set up a base URL to use for all subsequent requests. This is absolutely required.

```
$ ./http url https://api.travis-ci.com/
export HTTPCLI_URL=$'https://api.travis-ci.com/'
$ env | grep HTTPCLI_
(nothing here)
$
```

Hmm... That wasn't too useful. Why'd it output an `export` statement? Why didn't it modify the shell environment?

It seems that a process cannot modify its parent process's environment. In this context, that rule means that my `http` tool cannot modify the shell's environment directly. We have to resort to outputting bash commands to modify the shell's environment.

Let's try this instead:
```
$ eval `./http url https://api.travis-ci.com/`
$ env | grep HTTPCLI_
HTTPCLI_URL=https://api.travis-ci.com/
$
```

There we go. The trick is to use the `eval` statement to evaluate the output of the script in the current environment.

Let's confirm that base URL via the `http` tool itself:
```
$ ./http url
https://api.travis-ci.com/
```

The environment is now set up such that all subsequent requests are made to URLs that begin with this absolute URL.

Now let's set up some common headers that Travis requires:

```
$ eval `./http set Accepts application/vnd.travis-ci.2+json`
$ eval `./http set User-Agent http/0.1`
```

Travis requires these two headers at the bare minimum for all requests, regardless of authorization. User-Agent can be whatever you want.

In order to do anything remotely interesting with Travis's API, we need to authorize ourselves to their API to prove we have an account there.

Travis is tightly integrated with GitHub and even delegates to GitHub for authorization. According to Travis's docs, we need to acquire a GitHub personal access token (via GitHub's website) and convert that into a Travis authorization token using Travis's API.

```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | ./http post auth/github application/json
```

As you can see, an `echo` statement appears first because we need to send the POST body data via `stdin` to `http`. We do this by piping (`|`) the `stdout` of the `echo` process to the `stdin` of the `http` process.

We use the `http`'s `post` command which sends an HTTP POST request to the API server.

The first argument is the relative URL `auth/github` to POST the request to; this is relative to the absolute base URL we supplied earlier and set in the shell's environment, composing to the final URL `https://api.travis-ci.com/auth/github`.

Last is the optional `content-type` argument which is explcitly set to `application/json` here, but `application/json` is the default `content-type` anyway so it's a bit redundant in this example.

Let's see what Travis API responds with:

```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | ./http post auth/github application/json
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
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | ./http post auth/github 2>/dev/null
{"access_token":"access-token-REDACTED"}$ _cursor here_
```

As a matter of priniciple, `http` never outputs anything (not even extra trailing newlines) to `stdout` that did not come directly from the HTTP response body. That's why the shell '$' sigil appears on the same line as the end of the JSON content; there was no extra newline outputted.

Back to our API example...

Now that we have an access token from Travis, we should set that into the "Authorization" header that Travis requires for authorized requests:
```
$ ./http set Authorization "token \"access-token-REDACTED\""
export HTTPCLI_HEADER_Authorization=$'token "access-token-REDACTED"'
```

Crap, we gotta do that annoying `eval` workaround...
```
$ eval `./http set Authorization "token \"access-token-REDACTED\""`
```

Note that Travis requires literal double-quotes surrounding the access token.

Now that we're authorized let's check out some caches:
```
$ ./http get repos/redacted-org-name/redacted-repo-name/caches?branch=master
GET https://api.travis-ci.com/redacted-org-name/redacted-repo-name/caches?branch=master
Accepts: application/vnd.travis-ci.2+json
User-Agent: http/0.1
Authorization: token "access-token-REDACTED"

Sending HTTP request...

StatusCode: 200
{"caches":[{"repository_id":0,"size":207755213,"slug":"cache--python-2.7","branch":"master","last_modified":"2015-02-27T18:56:19Z"}]}
```

Let's delete that cache:
```
$ ./http delete repos/redacted-org-name/redacted-repo-name/caches?branch=master
```

Simple!

What if you've set up a complex `http` environment that you want to recall later? Easy:

```
$ ./http env
export HTTPCLI_URL=$'https://api.travis-ci.com/'
export HTTPCLI_HEADER_Authorization=$'token "access-token-REDACTED"'
export HTTPCLI_HEADER_Accepts=$'application/vnd.travis-ci.2+json'
export HTTPCLI_HEADER_User_Agent=$'http-cli/0.1'
```

Just redirect that `stdout` to a local file, `chmod +x` it and run it later to set up your environment again.

# How it works

This tool (`http`) relies on environment variables to minimize command invocation boilerplate. Have a common set of HTTP headers you need to pass for every request? Just set an environment variable for each one in your shell and `http` will pick those up and pass them along in the request.

The downside of using environment variables to maintain state is that a process cannot modify its parent process's environment. This means that in common shell contexts, any application cannot modify the shell's environment. There is a hacky workaround for this, and it's to emit a series of shell script statements to be `eval`uated by the parent shell.

```
eval `./http set User-Agent my-custom-agent/0.1`
```

The output of the 'set' command used above would be:

```
export HTTPCLI_HEADER_User_Agent=$'my-custom-agent/0.1'
```

RANT: Coming from a DOS background, personally, I think this is asinine, but of course I do see the security value in blocking such an ability in a POSIX system. Still, forcing the user to invoke my process with an eval wrapper is just ridiculous. If anyone knows of a more standard and cross-platform way to store global state without resorting to needlessly complex multi-process architectures involving busses or shared memory, I'm all ears. At the end of the day, I just want a way to store some simple variable in the shell context directly from within my Go process and pick it up later, without having to make my user jump through stupid hoops.

# Justification

I started this project as the ultimate result of a bit of yak-shaving. I was researching how to clear out [Travis CI](http://travis-ci.org/)'s caches and came across their HTTP API for doing so. My first instinct was to attempt HTTP API calls from the OS X commandline using `curl`. Of course, after a bit of testing, it seems standard `curl` on OS X does not have HTTPS enabled. Fail.

Next up was `wget`, which does work and supports HTTPS out of the box. I tried to wrap up all the wget boilerplate into a simple bash script, but failed because bash scripting is completely unreasonable and insane. Bash makes basic string manipulation and quoting so incredibly difficult and obtuse that I just gave up. I gave it several honest tries and nothing was working; the bash man page was no help either.

Even if I did manage to wrap wget in a script, its output is not flexible enough for programmatic control (for my tastes). It seems more fit for single-use requests made by humans than multi-use requests made by programs.

I decided I can do a better job writing a small tool in Go that only does HTTP API invocations in an extremely simple and direct way, consistent with how modern HTTP APIs are implemented and what they expect. Making the common thing easy to do is critical in reducing overall friction in any process.
