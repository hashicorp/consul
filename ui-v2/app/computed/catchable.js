import { computed as computedPropertyFactory } from '@ember/object';

export const computed = function() {
  const prop = computedPropertyFactory(...arguments);
  prop.catch = function(cb) {
    return this.meta({
      catch: cb,
    });
  };
  return prop;
};
