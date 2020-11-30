export default {
  Name: (item, value) => {
    return item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1;
  },
  Tags: (item, value) =>
    (item.Service.Tags || []).some(item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1),
  ID: (item, value) => (item.Service.ID || '').toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Address: (item, value) =>
    item.Address.toString()
      .toLowerCase()
      .indexOf(value.toLowerCase()) !== -1,
  Port: (item, value) =>
    item.Service.Port.toString()
      .toLowerCase()
      .indexOf(value.toLowerCase()) !== -1,
  ['Service.Meta']: (item, value) =>
    Object.entries(item.Meta || item.Service.Meta || {}).some(entry =>
      entry.some(item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1)
    ),
  ['Node.Meta']: (item, value) =>
    Object.entries(item.Node.Meta || {}).some(entry =>
      entry.some(item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1)
    ),
};
