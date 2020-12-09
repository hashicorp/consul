import { get } from '@ember/object';
export default function(foreignKey, nspaceKey, hash = JSON.stringify) {
  return function(primaryKey, slugKey, foreignKeyValue) {
    if (foreignKeyValue == null || foreignKeyValue.length < 1) {
      throw new Error('Unable to create fingerprint, missing foreignKey value');
    }
    return function(item) {
      const slugKeys = slugKey.split(',');
      const slugValues = slugKeys.map(function(slugKey) {
        if (get(item, slugKey) == null || get(item, slugKey).length < 1) {
          throw new Error('Unable to create fingerprint, missing slug');
        }
        return get(item, slugKey);
      });
      const nspaceValue = get(item, nspaceKey) || 'default';
      if (typeof item[nspaceKey] === 'undefined') {
        item[nspaceKey] = nspaceValue;
      }
      if (typeof item[foreignKey] === 'undefined') {
        item[foreignKey] = foreignKeyValue;
      }
      if (typeof item[primaryKey] === 'undefined') {
        item[primaryKey] = hash([nspaceValue, foreignKeyValue].concat(slugValues));
      }
      return item;
    };
  };
}
