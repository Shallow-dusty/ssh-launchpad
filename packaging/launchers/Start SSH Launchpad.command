#!/bin/sh
cd "$(dirname "$0")" || exit 2
export SSH_LAUNCHPAD_LAUNCHER=1
exec ./ssh-launchpad --interactive --lang auto
