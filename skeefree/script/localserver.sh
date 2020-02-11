#!/bin/bash -e

script/build


export APPBIN=$GOBIN/skeefree

export INTERNAL_ADDR="$(hostname):8080"

export CHATOPS_AUTH_PUBLIC_KEY=""

export CHATOPS_AUTH_BASE_URL="http://$(hostname):8080"

export GITHUB_API_BASE_URL="stuff"

echo "Booting server on ${INTERNAL_ADDR}..."

bin/skeefree
