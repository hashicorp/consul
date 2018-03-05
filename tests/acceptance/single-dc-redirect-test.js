import { test } from 'qunit';
import moduleForAcceptance from 'consul-ui/tests/helpers/module-for-acceptance';

moduleForAcceptance('Acceptance | single dc redirect');

test('visiting /', function(assert) {
  visit('/');
  andThen(function() {
    assert.equal(currentURL(), '/dc1/services');
  });
});
