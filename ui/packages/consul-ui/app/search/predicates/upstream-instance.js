export default {
  DestinationName: (item, value) =>
    item.DestinationName.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  LocalBindAddress: (item, value) =>
    item.LocalBindAddress.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  LocalBindPort: (item, value) =>
    item.LocalBindPort.toString()
      .toLowerCase()
      .indexOf(value.toLowerCase()) !== -1,
};
