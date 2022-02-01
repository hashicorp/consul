#!/bin/bash

if [ -d "/run/systemd/system" ]; then
    systemctl --system daemon-reload >/dev/null || :
fi

case "$1" in
    purge | 0)
        userdel consul
        ;;

    upgrade | [1-9]*)
        if [ -d "/run/systemd/system" ]; then
            systemctl try-restart consul.service >/dev/null || :
        fi
        ;;
esac

exit 0
