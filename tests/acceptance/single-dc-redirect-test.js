import { test } from 'qunit';
import moduleForAcceptance from 'consul-ui/tests/helpers/module-for-acceptance';

moduleForAcceptance('Acceptance | single dc redirect');

test('visiting / with only one datacenter redirects to the datacenter services page', function(assert) {
  const dcs = window.server.createList('dc', 1);
  visit('/');
  andThen(function() {
    assert.equal(currentURL(), `/${encodeURIComponent(dcs[0].Name)}/services`);
  });
});
test('visiting / with multiple datacenters shows a datacenter selection page', function(assert) {
  window.server.createList('dc', 2);
  visit('/');
  andThen(function() {
    assert.equal(currentURL(), '/');
  });
});
