export default {
  statuses: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
  },
  kinds: {
    service: (item, value) => item.Kind === value,
    node: (item, value) => item.Kind === value,
  },
  checks: {
    serf: (item, value) => item.Type === '',
    script: (item, value) => item.Type === value,
    http: (item, value) => item.Type === value,
    tcp: (item, value) => item.Type === value,
    ttl: (item, value) => item.Type === value,
    docker: (item, value) => item.Type === value,
    grpc: (item, value) => item.Type === value,
    alias: (item, value) => item.Type === value,
  },
};
