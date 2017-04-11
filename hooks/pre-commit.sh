#!/usr/bin/env bash
# maestro
# https://github.com/topfreegames/maestro
#
# Licensed under the MIT license:
# http://www.opensource.org/licenses/mit-license
# Copyright © 2017 Top Free Games <backend@tfgco.com>

# Redirect output to stderr.
exec 1>&2
# enable user input
exec < /dev/tty

forbiddenregexp='^\+.*[XF](It|Describe)[(]'
# CHECK
if test $(git diff --cached | egrep $forbiddenregexp | wc -l) != 0
then
    echo "Proposed diff:"
    exec git diff --cached | egrep -ne $forbiddenregexp
    echo
    echo "In the above diff, there's at least one occurrence of:"
    echo "    * XIt;"
    echo "    * FIt;"
    echo "    * XDescribe;"
    echo "    * FDescribe."
    echo
    echo "Please remove it before continuing!"
    exit 1;
fi
