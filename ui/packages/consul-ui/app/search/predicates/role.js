export default {
  Name: (item) => item.Name,
  Description: (item) => item.Description,
  Policy: (item) => {
    return (item.Policies || [])
      .map((item) => item.Name)
      .concat((item.ServiceIdentities || []).map((item) => item.ServiceName))
      .concat((item.NodeIdentities || []).map((item) => item.NodeName));
  },
};
