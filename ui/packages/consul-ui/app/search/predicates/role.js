export default {
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Description: (item, value) => item.Description.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Policy: (item, value) => {
    return (
      (item.Policies || []).some(
        item => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1
      ) ||
      (item.ServiceIdentities || []).some(
        item => item.ServiceName.toLowerCase().indexOf(value.toLowerCase()) !== -1
      ) ||
      (item.NodeIdentities || []).some(
        item => item.NodeName.toLowerCase().indexOf(value.toLowerCase()) !== -1
      )
    );
  },
};
