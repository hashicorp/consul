export default {
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
};
