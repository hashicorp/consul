export default {
  Node: (item, value) => item.Node.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Address: (item, value) => item.Address.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Meta: (item, value) =>
    Object.entries(item.Meta || {}).some(entry =>
      entry.some(item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1)
    ),
};
