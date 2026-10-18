#!/bin/sh
# Deploy to production through webmaster, which runs the OpenBSD server: it
# owns pf, relayd, httpd, acme-client, syslog, newsyslog and the rc.d script
# there, and comes back to this repo for the tests, the build and the binary
# swap (deploy/build.sh and deploy/push.sh openbsd HOST).
#
#   deploy/production.sh -n        # dry run: tests, build, checks the server, changes nothing
#   deploy/production.sh           # new binary + restart, rolled back on a failed health check
#   deploy/production.sh -S        # first setup or a server config change (webmaster's server/)
#   deploy/production.sh -h        # webmaster's own help (-f allows uncommitted changes)
#
# Settings (environment):
#   WEBMASTER_DIR   the webmaster checkout (default: ../webmaster next to this repo)
#
# Runs interactively only: doas on the server asks for a password.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
WEBMASTER_DIR=${WEBMASTER_DIR:-$(dirname "$root")/webmaster}
push_site=$WEBMASTER_DIR/bin/push-site
[ -x "$push_site" ] || {
	echo "webmaster not found at $WEBMASTER_DIR (set WEBMASTER_DIR to its checkout)" >&2
	exit 1
}

# webmaster builds from its own idea of where this repo is; make sure it's this one.
export WINTERMUTE_SRC="$root"
# The tests in production_test.go check webmaster's server config against
# what the site promises; this tells them where it is.
export WEBMASTER_DIR
exec "$push_site" "$@" wintermuteconsulting
