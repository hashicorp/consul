import writable from 'consul-ui/utils/model/writable';
import { module, test } from 'qunit';

module('Unit | Utility | model/writable');

test('it correctly marks attrs as serialize:true|false', function(assert) {
  const yes = {
    Props: true,
    That: true,
    Should: true,
    Be: true,
    Writable: true,
  };
  const no = {
    Others: true,
    Read: true,
    Only: true,
  };
  const expectedYes = Object.keys(yes);
  const expectedNo = Object.keys(no);
  const model = {
    eachAttribute: function(cb) {
      expectedYes.concat(expectedNo).forEach(function(item) {
        cb(item, {}); // we aren't testing the meta here, just use the same api
      });
    },
  };
  let attrs = writable(model, Object.keys(yes));
  const actualYes = Object.keys(attrs).filter(item => attrs[item].serialize);
  const actualNo = Object.keys(attrs).filter(item => !attrs[item].serialize);
  assert.deepEqual(actualYes, expectedYes, 'writable props are marked as serializable');
  assert.deepEqual(actualNo, expectedNo, 'writable props are marked as not serializable');
  attrs = writable(model, Object.keys(yes), {
    Props: {
      another: 'property',
    },
  });
  assert.equal(
    attrs.Props.another,
    'property',
    'previous attrs objects can be passed without being overwritten'
  );
});
