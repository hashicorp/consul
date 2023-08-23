export default function(visitable, clickable, attribute, collection) {
  return {
    visit: visitable('/:dc/'),
    dcs: collection('[data-test-datacenter-picker]'),
    showDatacenters: clickable('[data-test-datacenter-selected]'),
    selectedDc: attribute('data-test-datacenter-selected', '[data-test-datacenter-selected]'),
    selectedDatacenter: attribute(
      'data-test-datacenter-selected',
      '[data-test-datacenter-selected]'
    ),
  };
}
