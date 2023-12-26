#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

cat << EOF
## Changelog

TODO

## Downloads

- [Mac OSX 64bit](https://s3.amazonaws.com/downloads.wercker.com/cli/versions/$WERCKER_VERSION/darwin_amd64/wercker)
- [Linux 64bit](https://s3.amazonaws.com/downloads.wercker.com/cli/versions/$WERCKER_VERSION/linux_amd64/wercker)

## Checksums

- wercker_darwin_amd64:
  - sha256: \`$(awk '{print $1}' "$DIR/../artifacts/latest/darwin_amd64/SHA256SUMS")\`
- wercker_linux_amd64:
  - sha256: \`$(awk '{print $1}' "$DIR/../artifacts/latest/linux_amd64/SHA256SUMS")\`
EOF
