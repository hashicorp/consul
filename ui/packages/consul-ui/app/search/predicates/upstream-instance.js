export default {
  DestinationName: (item, value) => item.DestinationName,
  LocalBindAddress: (item, value) => item.LocalBindAddress,
  LocalBindPort: (item, value) => item.LocalBindPort.toString(),
};
