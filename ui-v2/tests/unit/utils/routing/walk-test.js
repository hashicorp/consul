import { walk } from 'consul-ui/utils/routing/walk';
import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Unit | Utility | routing/walk', function() {
  test('it walks down deep routes', function(assert) {
    const route = this.stub();
    const Router = {
      route: function(name, options, cb) {
        route();
        if (cb) {
          cb.apply(this, []);
        }
      },
    };
    walk.apply(Router, [
      {
        route: {
          _options: {
            path: '/:path',
          },
          next: {
            _options: {
              path: '/:path',
            },
            inside: {
              _options: {
                path: '/*path',
              },
            },
          },
        },
      },
    ]);
    assert.equal(route.callCount, 3);
  });
});
