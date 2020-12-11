export default {
  kinds: {
    management: (item, value) => item.Type === value,
    client: (item, value) => item.Type === value,
  },
};
