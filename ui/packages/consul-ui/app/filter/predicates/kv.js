import { andOr } from 'consul-ui/utils/filter';

export default andOr({
  kinds: {
    folder: (item, value) => item.isFolder,
    key: (item, value) => !item.isFolder,
  },
});
