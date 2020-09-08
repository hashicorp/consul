#!/bin/bash
# Configure systemd-resolved it to forward requests for a specific domain to Consul. This script has been tested
# with the following operating systems:
#
# 1. Ubuntu 18.04
# See https://learn.hashicorp.com/consul/security-networking/forwarding#systemd-resolved-setup for more details
# Github Issue: https://github.com/hashicorp/consul/issues/4155

set -e

readonly DEFAULT_CONSUL_DOMAIN="consul"
readonly DEFAULT_CONSUL_IP="127.0.0.1"
readonly DEFAULT_CONSUL_DNS_PORT=8600

readonly SYSTEMD_RESVOLDED_CONFIG_FILE="/etc/systemd/resolved.conf"

readonly SCRIPT_NAME="$(basename "$0")"

function print_usage {
  echo
  echo "Usage: setup-systemd-resolved [OPTIONS]"
  echo
  echo "Configure systemd-resolved to forward requests for a specific domain to Consul. This script has been tested with Ubuntu 18.04."
  echo
  echo "Options:"
  echo
  echo -e "  --consul-domain\tThe domain name to point to Consul. Optional. Default: $DEFAULT_CONSUL_DOMAIN."
  echo -e "  --consul-ip\t\tThe IP address to use for Consul. Optional. Default: $DEFAULT_CONSUL_IP."
  echo -e "  --consul-dns-port\tThe port Consul uses for DNS. Optional. Default: $DEFAULT_CONSUL_DNS_PORT."
  echo
  echo "Example:"
  echo
  echo "  setup-systemd-resolved"
}

function log {
  local -r level="$1"
  local -r message="$2"
  local -r timestamp=$(date +"%Y-%m-%d %H:%M:%S")
  >&2 echo -e "${timestamp} [${level}] [$SCRIPT_NAME] ${message}"
}

function log_info {
  local -r message="$1"
  log "INFO" "$message"
}

function log_warn {
  local -r message="$1"
  log "WARN" "$message"
}

function log_error {
  local -r message="$1"
  log "ERROR" "$message"
}

function assert_not_empty {
  local -r arg_name="$1"
  local -r arg_value="$2"

  if [[ -z "$arg_value" ]]; then
    log_error "The value for '$arg_name' cannot be empty"
    print_usage
    exit 1
  fi
}

function install_dependencies {
  local -r consul_ip="$1"

  log_info "Installing dependencies"
  sudo apt-get update -y
  echo iptables-persistent iptables-persistent/autosave_v4 boolean true | sudo debconf-set-selections
  echo iptables-persistent iptables-persistent/autosave_v6 boolean true | sudo debconf-set-selections
  sudo apt-get install -y iptables-persistent
}

function configure_systemd_resolved {
  local -r consul_domain="$1"
  local -r consul_ip="$2"
  local -r consul_port="$3"

  UBUNTU_VERSION=`lsb_release -s -r`
  if [ "$UBUNTU_VERSION" == "18.04" ]; then
    log_info "Configuring systemd-resolved to forward lookups of the '$consul_domain' domain to $consul_ip:$consul_port in $CONSUL_DNS_MASQ_CONFIG_FILE"

    sudo iptables -t nat -A OUTPUT -d localhost -p udp -m udp --dport 53 -j REDIRECT --to-ports $consul_port
    sudo iptables -t nat -A OUTPUT -d localhost -p tcp -m tcp --dport 53 -j REDIRECT --to-ports $consul_port
    sudo iptables-save | sudo tee /etc/iptables/rules.v4
    sudo ip6tables-save | sudo tee /etc/iptables/rules.v6
    sudo sed -i "s/#DNS=/DNS=${consul_ip}/g" "$SYSTEMD_RESVOLDED_CONFIG_FILE"
    sudo sed -i "s/#Domains=/Domains=~${consul_domain}/g" "$SYSTEMD_RESVOLDED_CONFIG_FILE"
  else
    log_error "Cannot install on this OS."
    exit 1
  fi
}

function install {
  local consul_domain="$DEFAULT_CONSUL_DOMAIN"
  local consul_ip="$DEFAULT_CONSUL_IP"
  local consul_dns_port="$DEFAULT_CONSUL_DNS_PORT"

  while [[ $# > 0 ]]; do
    local key="$1"

    case "$key" in
      --consul-domain)
        assert_not_empty "$key" "$2"
        consul_domain="$2"
        shift
        ;;
      --consul-ip)
        assert_not_empty "$key" "$2"
        consul_ip="$2"
        shift
        ;;
      --consul-dns-port)
        assert_not_empty "$key" "$2"
        consul_dns_port="$2"
        shift
        ;;
      --help)
        print_usage
        exit
        ;;
      *)
        log_error "Unrecognized argument: $key"
        print_usage
        exit 1
        ;;
    esac

    shift
  done

  log_info "Configuring systemd-resolved"
  install_dependencies
  configure_systemd_resolved "$consul_domain" "$consul_ip" "$consul_dns_port"
  log_info "systemd-resolved configured!"
}

install "$@"
