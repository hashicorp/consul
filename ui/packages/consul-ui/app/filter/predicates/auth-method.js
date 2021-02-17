export default {
  kind: {
    kubernetes: (item, value) => item.Type === value,
    jwt: (item, value) => item.Type === value,
    oidc: (item, value) => item.Type === value,
  },
  source: {
    local: (item, value) => item.TokenLocality === value,
    global: (item, value) => item.TokenLocality === value,
  },
};
