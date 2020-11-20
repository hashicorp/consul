export default {
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Tags: (item, value) => (item.Tags || []).some(item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1)
};
