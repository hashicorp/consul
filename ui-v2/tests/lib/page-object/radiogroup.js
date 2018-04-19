import { is, clickable } from 'ember-cli-page-object';
import ucfirst from 'consul-ui/utils/ucfirst';
export default function(name, items) {
  return items.reduce(function(prev, item, i, arr) {
    // if item is empty then it means 'all'
    // otherwise camelCase based on something-here = somethingHere for the key
    const key =
      item === ''
        ? 'all'
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
          `[data-test-radiobutton="${name}_${item}"] > input[type="radio"]`
        ),
        [key]: clickable(`[data-test-radiobutton="${name}_${item}"]`),
      },
    };
  }, {});
}
