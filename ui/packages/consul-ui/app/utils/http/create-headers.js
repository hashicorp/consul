export default function() {
  return function(lines) {
    return lines.reduce(function(prev, item) {
      const [key, ...value] = item.split(':');
      if (value.length > 0) {
        prev[key.trim()] = value.join(':').trim();
      }
      return prev;
    }, {});
  };
}
