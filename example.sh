#!/bin/bash

if [ -z "$1" ]; then
  echo "Please supply a 40-digit GitHub access token for this example."
  exit
fi

echo "Session ID for this script:"
http session
echo

http set Accepts application/vnd.travis-ci.2+json
http set User-Agent http-cli/0.1
http url https://api.travis-ci.com
echo -n "{\"github_token\":\"$1\"}" | http post auth/github 2>/dev/null
echo

if (( $? == 0 )); then
	echo Success
else
	echo Failed
fi
