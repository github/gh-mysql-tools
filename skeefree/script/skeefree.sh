#!/bin/bash

[ -f /etc/profile.d/skeefree.sh ] && . /etc/profile.d/skeefree.sh

# skeefree, being a mu app, needs to get HTTP ports.
# In CLI mode, nothing actually connects to these ports.
# 8222 and 8223 are just dummy values.
$(dirname $0)/skeefree -http-addr ":8222" -internal-addr ":8223" "$@"
