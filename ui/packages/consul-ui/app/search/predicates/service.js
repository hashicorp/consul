export default {
  Name: (item) => item.Name,
  Tags: (item) => item.Tags || [],
  PeerName: (item) => item.PeerName,
  Partition: (item) => item.Partition,
};
