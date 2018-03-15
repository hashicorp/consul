import { module } from 'ember-qunit';
// import Service from '@ember/service';
import test from 'ember-sinon-qunit/test-support/test';
import transitionToNearestParent from 'consul-ui/utils/transitionToNearestParent';
module('Unit | Utils | transitionToNearestParent', {});

test('it transitions to root when parent is "/"', function(assert) {
  const expected = 'root';
  const res = transitionToNearestParent.bind({
    transitionTo: function(route, actual) {
      assert.equal(actual, expected);
      return true;
    },
  })('dc', '/', expected);
  res.then(function(res) {
    assert.ok(res);
  });
});
test('it transitions to parent when parent exists', function(assert) {
  const expected = 'parent';
  const res = transitionToNearestParent.bind({
    transitionTo: function(route, actual) {
      assert.equal(actual, expected);
      return true;
    },
    get: function() {
      return {
        findAllBySlug: function(actual) {
          assert.equal(actual, expected);
          return Promise.resolve(null);
        },
      };
    },
  })('dc', expected, 'root');
  res.then(function(res) {
    assert.ok(res);
  });
});
test('it transitions to root when parent does not exist', function(assert) {
  const expectedSlug = 'parent';
  const expected = 'root';
  const res = transitionToNearestParent.bind({
    transitionTo: function(route, actual) {
      assert.equal(actual, expected);
      return true;
    },
    get: function() {
      return {
        findAllBySlug: function(actual) {
          assert.equal(actual, expectedSlug);
          return Promise.reject({
            errors: [
              {
                status: 404,
              },
            ],
          });
        },
      };
    },
  })('dc', expectedSlug, expected);
  res.then(function(res) {
    assert.ok(res);
  });
});
