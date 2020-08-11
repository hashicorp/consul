export default function(foreignKey, nspaceKey, hash = JSON.stringify) {
  return function(primaryKey, slugKey, foreignKeyValue) {
    if (foreignKeyValue == null || foreignKeyValue.length < 1) {
      throw new Error('Unable to create fingerprint, missing foreignKey value');
    }
    return function(item) {
      const slugKeys = slugKey.split(',');
      const slugValues = slugKeys.map(function(slugKey) {
        if (item[slugKey] == null || item[slugKey].length < 1) {
          throw new Error('Unable to create fingerprint, missing slug');
        }
        return item[slugKey];
      });
      const nspaceValue = item[nspaceKey] || 'default';
      return {
        ...item,
        ...{
          [nspaceKey]: nspaceValue,
          [foreignKey]: foreignKeyValue,
          [primaryKey]: hash([nspaceValue, foreignKeyValue].concat(slugValues)),
        },
      };
    };
  };
}
