export default function (visitable, collection) {
  return {
    visit: visitable('/'),
    dcs: collection('[data-test-datacenter-list]'),
  };
}
