# http-cli
CLI for invoking HTTP APIs

# Justification

I started this project as the ultimate result of a bit of yak-shaving. I was researching how to clear out [Travis CI](http://travis-ci.org/)'s caches and came across their HTTP API for doing so. My first instinct was to attempt HTTP API calls from the OS X commandline using `curl`. Of course, after a bit of testing, it seems standard `curl` on OS X does not have HTTPS enabled. Fail.

`wget` does work, actually. I tried to wrap up the wget boilerplate into a simple bash script, but failed because bash scripting is completely unreasonable and insane. All I wanted to do was to wrap up the wget call with the boilerplate headers and such, but bash makes basic string manipulation so incredibly difficult and obtuse that I just gave up. I gave it several honest tries and nothing was working. I couldn't even debug what was going wrong or see what it thought it was executing on the shell with the strings I was handing it for arguments.

I can do a better job with Go to make a generic HTTP API invoker making use of environment variables to maintain state like for authentication tokens.
