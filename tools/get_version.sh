#!/bin/bash

if [ "$GITHUB_REF" == "" ]; then
  VERSION="v0.0.1"
else
  VERSION=`basename $GITHUB_REF`
fi

echo -n $VERSION >internal/types/version.txt