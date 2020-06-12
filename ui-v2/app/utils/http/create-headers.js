export default function() {
  return function(lines) {
    return lines.reduce(function(prev, item) {
      const temp = item.split(':');
      if (temp.length > 1) {
        prev[temp[0].trim()] = temp[1].trim();
      }
      return prev;
    }, {});
  };
}
