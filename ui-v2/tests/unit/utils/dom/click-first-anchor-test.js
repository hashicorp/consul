import domClickFirstAnchor from 'consul-ui/utils/dom/click-first-anchor';
import { module, test } from 'qunit';

module('Unit | Utility | dom/click first anchor');

test('it does nothing if the clicked element is generally a clickable thing', function(assert) {
  const closest = function() {
    return {
      querySelector: function() {
        assert.ok(false);
      },
    };
  };
  const click = domClickFirstAnchor(closest);
  ['INPUT', 'LABEL', 'A', 'Button'].forEach(function(item) {
    const expected = null;
    const actual = click({
      target: {
        nodeName: item,
      },
    });
    assert.equal(actual, expected);
  });
});
test("it does nothing if an anchor isn't found", function(assert) {
  const closest = function() {
    return {
      querySelector: function() {
        return null;
      },
    };
  };
  const click = domClickFirstAnchor(closest);
  const expected = null;
  const actual = click({
    target: {
      nodeName: 'DIV',
    },
  });
  assert.equal(actual, expected);
});
test('it dispatches the result of `click` if an anchor is found', function(assert) {
  assert.expect(1);
  const expected = 'click';
  const closest = function() {
    return {
      querySelector: function() {
        return {
          dispatchEvent: function(ev) {
            const actual = ev.type;
            assert.equal(actual, expected);
          },
        };
      },
    };
  };
  const click = domClickFirstAnchor(closest);
  click({
    target: {
      nodeName: 'DIV',
    },
  });
});
