export default {
  Node: item => item.Node,
  Address: item => item.Address,
  Meta: item => Object.entries(item.Meta || {}).reduce((prev, entry) => prev.concat(entry), []),
};
