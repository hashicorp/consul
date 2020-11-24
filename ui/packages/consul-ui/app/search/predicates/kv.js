import rightTrim from 'consul-ui/utils/right-trim';
export default {
  Key: (item, value) =>
    rightTrim(item.Key.toLowerCase())
      .split('/')
      .pop()
      .indexOf(value.toLowerCase()) !== -1,
};
