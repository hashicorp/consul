import { is, clickable } from 'ember-cli-page-object';
import ucfirst from 'consul-ui/utils/ucfirst';
// TODO: We no longer need to use name here
// remove the arg in all objects
export default function(name, items, blankKey = 'all') {
  return items.reduce(function(prev, item, i, arr) {
    // if item is empty then it means 'all'
    // otherwise camelCase based on something-here = somethingHere for the key
    const key =
      item === ''
        ? blankKey
        : item.split('-').reduce(function(prev, item, i, arr) {
            if (i === 0) {
              return item;
            }
            return prev + ucfirst(item);
          });
    return {
      ...prev,
      ...{
        [`${key}IsSelected`]: is(
          ':checked',
          `[data-test-radiobutton$="_${item}"] > input[type="radio"]`
        ),
        [key]: clickable(`[data-test-radiobutton$="_${item}"]`),
      },
    };
  }, {});
}
