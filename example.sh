#!/bin/bash

. http set Accepts application/vnd.travis-ci.2+json
. http set User-Agent http-cli/0.1
. http url https://api.travis-ci.com
echo -n '{"github_token":"A-40-CHARACTER-HEXADECIMAL-GITHUB-TOKEN-"}' | . http post auth/github
