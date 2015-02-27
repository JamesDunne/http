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

First, let's set up a base URL to use for all subsequent requests:
```
$ ./http url https://api.travis-ci.com/
```

Confirming that base URL:
```
$ ./http url
https://api.travis-ci.com/
```

Now, let's set up some common headers that we'll need. FYI, we're targeting the Travis CI build system's HTTP API as an example here.

```
$ ./http set Accepts application/vnd.travis-ci.2+json
$ ./http set User-Agent http-cli/0.1
```

Travis requires these two headers at the bare minimum for all requests, regardless of authorization.

Now, we need to convert a GitHub authorization token into a Travis authorization token. This is per the Travis API documentation.

```
$ echo -n '{"github_token":"really-long-hex-string-here-REDACTED"}' | ./http post auth/github application/json
```

As you can see, an `echo` statement appears first because we need to send the POST body data via `stdin` to `http`.

Our command is `post` which means to send a POST request to the API server. Following that, we supply a relative URL `auth/github`. Promptly following that is the Content-Type header for the POST body we're sending, which is `application/json` in this case.

Let's see what Travis API responds with:

```
POST https://api.travis-ci.com/auth/github
Content-Type: application/json
Accepts: application/vnd.travis-ci.2+json
User-Agent: http-cli/0.1

{"github_token":"really-long-hex-string-here-REDACTED"}
{"access_token":"shorter-token-REDACTED"}
```

That "access_token" line is the API response containing the token we need to set for future requests. The line above it is the POST body that was sent.

Let's set the "Authorization" header that Travis requires for authorized requests.

```
$ ./http set Authorization "token \"shorter-token-REDACTED\""
```

# What it does

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
