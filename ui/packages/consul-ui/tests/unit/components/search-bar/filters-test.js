import { filters } from 'consul-ui/components/search-bar/utils';
import { module, test } from 'qunit';

module('Unit | Component | search-bar/filters', function() {
  test('it correctly reshapes the filter data', function(assert) {
    [
      // basic filter, returns a single filter button when clicked
      // resets selected/queryparam to empty
      {
        filters: {
          status: {
            value: ['passing'],
          },
        },
        expected: [
          {
            key: 'status',
            value: 'passing',
            selected: [],
          },
        ],
      },
      // basic filters, returns multiple filter button when clicked
      // sets selected/queryparam to the left over single filter
      {
        filters: {
          status: {
            value: ['passing', 'warning'],
          },
        },
        expected: [
          {
            key: 'status',
            value: 'passing',
            selected: ['warning'],
          },
          {
            key: 'status',
            value: 'warning',
            selected: ['passing'],
          },
        ],
      },
      // basic filters, returns multiple filter button when clicked
      // sets selected/queryparam to the left over multiple filters
      {
        filters: {
          status: {
            value: ['passing', 'warning', 'critical'],
          },
        },
        expected: [
          {
            key: 'status',
            value: 'passing',
            selected: ['warning', 'critical'],
          },
          {
            key: 'status',
            value: 'warning',
            selected: ['passing', 'critical'],
          },
          {
            key: 'status',
            value: 'critical',
            selected: ['passing', 'warning'],
          },
        ],
      },
      // basic filters, returns multiple filter button when clicked
      // sets selected/queryparam to the left over multiple filters
      // also search property multiple filter, sets the selected/queryparam to
      // the left of single searchproperty filter
      {
        filters: {
          status: {
            value: ['passing', 'warning', 'critical'],
          },
          searchproperties: {
            default: ['Node', 'Address', 'Meta'],
            value: ['Node', 'Address'],
          },
        },
        expected: [
          {
            key: 'status',
            value: 'passing',
            selected: ['warning', 'critical'],
          },
          {
            key: 'status',
            value: 'warning',
            selected: ['passing', 'critical'],
          },
          {
            key: 'status',
            value: 'critical',
            selected: ['passing', 'warning'],
          },
          {
            key: 'searchproperties',
            value: 'Node',
            selected: ['Address'],
          },
          {
            key: 'searchproperties',
            value: 'Address',
            selected: ['Node'],
          },
        ],
      },
      // basic filters, returns multiple filter button when clicked
      // sets selected/queryparam to the left over multiple filters
      // also search property single filter, resets the selected/queryparam to
      // empty
      {
        filters: {
          status: {
            value: ['passing', 'warning', 'critical'],
          },
          searchproperties: {
            default: ['Node', 'Address', 'Meta'],
            value: ['Node'],
          },
        },
        expected: [
          {
            key: 'status',
            value: 'passing',
            selected: ['warning', 'critical'],
          },
          {
            key: 'status',
            value: 'warning',
            selected: ['passing', 'critical'],
          },
          {
            key: 'status',
            value: 'critical',
            selected: ['passing', 'warning'],
          },
          {
            key: 'searchproperties',
            value: 'Node',
            selected: [],
          },
        ],
      },
    ].forEach(item => {
      const actual = filters(item.filters);
      assert.deepEqual(actual, item.expected);
    });
  });
});
