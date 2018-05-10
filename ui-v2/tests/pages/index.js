import { create, visitable, collection } from 'ember-cli-page-object';

export default create({
  visit: visitable('/'),
  dcs: collection('[data-test-datacenter-list]'),
});
