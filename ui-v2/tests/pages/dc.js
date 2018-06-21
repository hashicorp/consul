import { create, visitable, attribute, collection, clickable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/'),
  dcs: collection('[data-test-datacenter-picker]'),
  showDatacenters: clickable('[data-test-datacenter-selected]'),
  selectedDc: attribute('data-test-datacenter-selected', '[data-test-datacenter-selected]'),
  selectedDatacenter: attribute('data-test-datacenter-selected', '[data-test-datacenter-selected]'),
});
