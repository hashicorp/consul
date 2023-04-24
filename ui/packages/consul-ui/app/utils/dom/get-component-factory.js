export default function(owner, key = '-view-registry:main') {
  const components = owner.lookup(key);
  return function(el) {
    const id = el.getAttribute('id');
    if (id) {
      return components[id];
    }
  };
}
