import { is, clickable, attribute, isVisible } from 'ember-cli-page-object';
import ucfirst from 'consul-ui/utils/ucfirst';
export default function (name, items, blankKey = 'all') {
  return items.reduce(function (prev, item, i, arr) {
    // if item is empty then it means 'all'
    // otherwise camelCase based on something-here = somethingHere for the key
    const key =
      item === ''
        ? blankKey
        : item.split('-').reduce(function (prev, item, i, arr) {
            if (i === 0) {
              return item;
            }
            return prev + ucfirst(item);
          });
    return {
      ...prev,
      ...{
        [`${key}IsSelected`]: is('.selected', `[data-test-tab="${name}_${item}"]`),
        [`${key}Url`]: attribute('href', `[data-test-tab="${name}_${item}"] a`),
        [key]: clickable(`[data-test-tab="${name}_${item}"] a`),
        [`${key}IsVisible`]: isVisible(`[data-test-tab="${name}_${item}"] a`),
      },
    };
  }, {});
}
