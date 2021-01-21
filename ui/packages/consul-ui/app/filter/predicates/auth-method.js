export default {
  types: {
    kubernetes: (item, value) => item.Type === value,
    jwt: (item, value) => item.Type === value,
    oidc: (item, value) => item.Type === value,
  },
};
