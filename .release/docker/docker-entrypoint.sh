#!/usr/bin/dumb-init /bin/sh
set -e

# Note above that we run dumb-init as PID 1 in order to reap zombie processes
# as well as forward signals to all processes in its session. Normally, sh
# wouldn't do either of these functions so we'd leak zombies as well as do
# unclean termination of all our sub-processes.
# As of docker 1.13, using docker run --init achieves the same outcome.

# You can set CONSUL_BIND_INTERFACE to the name of the interface you'd like to
# bind to and this will look up the IP and pass the proper -bind= option along
# to Consul.
if [ -z "$CONSUL_BIND" ]; then
  if [ -n "$CONSUL_BIND_INTERFACE" ]; then
    CONSUL_BIND_ADDRESS=$(ip -o -4 addr list $CONSUL_BIND_INTERFACE | head -n1 | awk '{print $4}' | cut -d/ -f1)
    if [ -z "$CONSUL_BIND_ADDRESS" ]; then
      echo "Could not find IP for interface '$CONSUL_BIND_INTERFACE', exiting"
      exit 1
    fi

    CONSUL_BIND="-bind=$CONSUL_BIND_ADDRESS"
    echo "==> Found address '$CONSUL_BIND_ADDRESS' for interface '$CONSUL_BIND_INTERFACE', setting bind option..."
  fi
fi

# You can set CONSUL_CLIENT_INTERFACE to the name of the interface you'd like to
# bind client intefaces (HTTP, DNS, and RPC) to and this will look up the IP and
# pass the proper -client= option along to Consul.
if [ -z "$CONSUL_CLIENT" ]; then
  if [ -n "$CONSUL_CLIENT_INTERFACE" ]; then
    CONSUL_CLIENT_ADDRESS=$(ip -o -4 addr list $CONSUL_CLIENT_INTERFACE | head -n1 | awk '{print $4}' | cut -d/ -f1)
    if [ -z "$CONSUL_CLIENT_ADDRESS" ]; then
      echo "Could not find IP for interface '$CONSUL_CLIENT_INTERFACE', exiting"
      exit 1
    fi

    CONSUL_CLIENT="-client=$CONSUL_CLIENT_ADDRESS"
    echo "==> Found address '$CONSUL_CLIENT_ADDRESS' for interface '$CONSUL_CLIENT_INTERFACE', setting client option..."
  fi
fi

# CONSUL_DATA_DIR is exposed as a volume for possible persistent storage. The
# CONSUL_CONFIG_DIR isn't exposed as a volume but you can compose additional
# config files in there if you use this image as a base, or use CONSUL_LOCAL_CONFIG
# below.
if [ -z "$CONSUL_DATA_DIR" ]; then
  CONSUL_DATA_DIR=/consul/data
fi

if [ -z "$CONSUL_CONFIG_DIR" ]; then
  CONSUL_CONFIG_DIR=/consul/config
fi

# You can also set the CONSUL_LOCAL_CONFIG environemnt variable to pass some
# Consul configuration JSON without having to bind any volumes.
if [ -n "$CONSUL_LOCAL_CONFIG" ]; then
	echo "$CONSUL_LOCAL_CONFIG" > "$CONSUL_CONFIG_DIR/local.json"
fi

# If the user is trying to run Consul directly with some arguments, then
# pass them to Consul.
if [ "${1:0:1}" = '-' ]; then
  set -- consul "$@"
fi

# Look for Consul subcommands.
if [ "$1" = 'agent' ]; then
  shift
  set -- consul agent \
    -data-dir="$CONSUL_DATA_DIR" \
    -config-dir="$CONSUL_CONFIG_DIR" \
    $CONSUL_BIND \
    $CONSUL_CLIENT \
    "$@"
elif [ "$1" = 'version' ]; then
  # This needs a special case because there's no help output.
  set -- consul "$@"
elif consul --help "$1" 2>&1 | grep -q "consul $1"; then
  # We can't use the return code to check for the existence of a subcommand, so
  # we have to use grep to look for a pattern in the help output.
  set -- consul "$@"
fi

# If we are running Consul, make sure it executes as the proper user.
if [ "$1" = 'consul' -a -z "${CONSUL_DISABLE_PERM_MGMT+x}" ]; then
  # Allow to setup user and group via envrironment
  if [ -z "$CONSUL_UID" ]; then
    CONSUL_UID="$(id -u consul)"
  fi

  if [ -z "$CONSUL_GID" ]; then
    CONSUL_GID="$(id -g consul)"
  fi

  # If the data or config dirs are bind mounted then chown them.
  # Note: This checks for root ownership as that's the most common case.
  if [ "$(stat -c %u "$CONSUL_DATA_DIR")" != "${CONSUL_UID}" ]; then
    chown ${CONSUL_UID}:${CONSUL_GID} "$CONSUL_DATA_DIR"
  fi
  if [ "$(stat -c %u "$CONSUL_CONFIG_DIR")" != "${CONSUL_UID}" ]; then
    chown ${CONSUL_UID}:${CONSUL_GID} "$CONSUL_CONFIG_DIR"
  fi

  # If requested, set the capability to bind to privileged ports before
  # we drop to the non-root user. Note that this doesn't work with all
  # storage drivers (it won't work with AUFS).
  if [ ! -z ${CONSUL_ALLOW_PRIVILEGED_PORTS+x} ]; then
    setcap "cap_net_bind_service=+ep" /bin/consul
  fi

  set -- su-exec ${CONSUL_UID}:${CONSUL_GID} "$@"
fi

exec "$@"
