export default function(foreignKey, nspaceKey, nspaceUndefinedName, hash = JSON.stringify) {
  return function(primaryKey, slugKey, foreignKeyValue) {
    if (foreignKeyValue == null || foreignKeyValue.length < 1) {
      throw new Error('Unable to create fingerprint, missing foreignKey value');
    }
    return function(item) {
      if (item[slugKey] == null || item[slugKey].length < 1) {
        throw new Error('Unable to create fingerprint, missing slug');
      }
      const nspaceValue = item[nspaceKey] || nspaceUndefinedName;
      return {
        ...item,
        ...{
          [nspaceKey]: nspaceValue,
          [foreignKey]: foreignKeyValue,
          [primaryKey]: hash([nspaceValue, foreignKeyValue, item[slugKey]]),
        },
      };
    };
  };
}
