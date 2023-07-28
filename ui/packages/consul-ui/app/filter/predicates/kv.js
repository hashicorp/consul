export default {
  kind: {
    folder: (item, value) => item.isFolder,
    key: (item, value) => !item.isFolder,
  },
};
