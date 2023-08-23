import { get } from '@ember/object';

export default function(foreignKey, nspaceKey, partitionKey, hash = JSON.stringify) {
  return function(primaryKey, slugKey, foreignKeyValue, nspaceValue, partitionValue) {
    return function(item) {
      foreignKeyValue = foreignKeyValue == null ? item[foreignKey] : foreignKeyValue;
      if (foreignKeyValue == null) {
        throw new Error(
          `Unable to create fingerprint, missing foreignKey value. Looking for value in \`${foreignKey}\` got \`${foreignKeyValue}\``
        );
      }
      const slugKeys = slugKey.split(',');
      const slugValues = slugKeys.map(function(slugKey) {
        const slug = get(item, slugKey);
        if (slug == null || slug.length < 1) {
          throw new Error(
            `Unable to create fingerprint, missing slug. Looking for value in \`${slugKey}\` got \`${slug}\``
          );
        }
        return slug;
      });
      // This ensures that all data objects have a Namespace and a Partition
      // value set, even in OSS.
      if (typeof item[nspaceKey] === 'undefined') {
        if (nspaceValue === '*') {
          nspaceValue = 'default';
        }
        item[nspaceKey] = nspaceValue;
      }
      if (typeof item[partitionKey] === 'undefined') {
        if (partitionValue === '*') {
          partitionValue = 'default';
        }
        item[partitionKey] = partitionValue;
      }

      if (typeof item[foreignKey] === 'undefined') {
        item[foreignKey] = foreignKeyValue;
      }
      if (typeof item[primaryKey] === 'undefined') {
        item[primaryKey] = hash(
          [item[partitionKey], item[nspaceKey], foreignKeyValue].concat(slugValues)
        );
      }
      return item;
    };
  };
}
