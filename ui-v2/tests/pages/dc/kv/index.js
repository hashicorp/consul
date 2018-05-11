import { create, visitable, collection, attribute } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/kv'),
  kvs: collection('[data-test-kv]', {
    name: attribute('data-test-kv'),
  }),
});
