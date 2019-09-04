export default function(foreignKey, hash = JSON.stringify) {
  return function(primaryKey, slugKey, foreignKeyValue) {
    if (foreignKeyValue == null || foreignKeyValue.length < 1) {
      throw new Error('Unable to create fingerprint, missing foreignKey value');
    }
    return function(item) {
      if (item[slugKey] == null || item[slugKey].length < 1) {
        throw new Error('Unable to create fingerprint, missing slug');
      }
      return {
        ...item,
        ...{
          [foreignKey]: foreignKeyValue,
          [primaryKey]: hash([foreignKeyValue, item[slugKey]]),
        },
      };
    };
  };
}
