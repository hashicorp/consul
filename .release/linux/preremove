#!/bin/bash
case "$1" in
    remove | 0)
        if [ -d "/run/systemd/system" ]; then
            systemctl --no-reload disable consul.service > /dev/null || :
            systemctl stop consul.service > /dev/null || :
        fi
        ;;
esac

exit 0
