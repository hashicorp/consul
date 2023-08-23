export default function(path, num) {
  const parts = path.split('/');
  return parts.length > num
    ? parts
        .slice(0, -num)
        .concat('')
        .join('/')
    : '';
}
