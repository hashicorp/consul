import { get, set } from '@ember/object';
import ObjectProxy from '@ember/object/proxy';
export default function(items, item, prop, value) {
  value = typeof value === 'undefined' ? get(item, prop) : value;
  const pos = items.findIndex(function(item) {
    return get(item, prop) === value;
  });
  if (pos !== -1) {
    // TODO: We only currently use this with EventSources
    // would be good to check this doesn't do anything unexpected
    // with other proxies.
    // Before we get there, we might have figured a better way to do
    // this anyway
    if (item instanceof ObjectProxy) {
      set(item, 'content', items.objectAt(pos));
    }
    items.replace(pos, 1, [item]);
    // TODO: Looks like with new proxies this isn't needed anymore
    // items.enumerableContentDidChange();
  }
  return item;
}
