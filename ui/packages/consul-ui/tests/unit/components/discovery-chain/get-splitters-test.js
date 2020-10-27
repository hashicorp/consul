import { getSplitters } from 'consul-ui/components/discovery-chain/utils';
import { module, test } from 'qunit';

module('Unit | Component | discovery-chain/get-splitters', function() {
  test('it collects and correctly parses splitter Names', function(assert) {
    const actual = getSplitters({
      'splitter:splitter-name.default': {
        Type: 'splitter',
        Name: 'splitter-name.default',
        Splits: [
          {
            Weight: 50,
            NextNode: '',
          },
          {
            Weight: 50,
            NextNode: '',
          },
        ],
      },
      'splitter:not-splitter-name.default': {
        Type: 'not-splitter',
        Name: 'splitter-name.default',
        Splits: [
          {
            Weight: 50,
            NextNode: '',
          },
          {
            Weight: 50,
            NextNode: '',
          },
        ],
      },
    });
    const expected = {
      Type: 'splitter',
      Name: 'splitter-name',
      ID: 'splitter:splitter-name.default',
      Splits: [
        {
          Weight: 50,
          NextNode: '',
        },
        {
          Weight: 50,
          NextNode: '',
        },
      ],
    };
    assert.equal(actual.length, 1);
    assert.deepEqual(actual[0], expected);
  });
});
