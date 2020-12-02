export default {
  kinds: {
    folder: (item, value) => item.isFolder,
    key: (item, value) => !item.isFolder,
  },
};
