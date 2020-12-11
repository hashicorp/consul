export default {
  accesses: {
    allow: (item, value) => item.Action === value,
    deny: (item, value) => item.Action === value,
    'app-aware': (item, value) => typeof item.Action === 'undefined',
  },
};
