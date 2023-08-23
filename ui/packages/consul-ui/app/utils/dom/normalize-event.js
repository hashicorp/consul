export default function(e, value, target = {}) {
  if (typeof e.target !== 'undefined') {
    return e;
  }
  return {
    target: { ...target, ...{ name: e, value: value } },
  };
}
