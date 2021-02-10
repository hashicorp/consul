import wayfarer from 'wayfarer';
import { singularize } from 'ember-inflector';

const router = wayfarer();
export default path => (target, propertyKey, desc) => {
  router.on(path, function(params, owner) {
    let name;
    if (params.modelName) {
      name = singularize(params.modelName);
    } else {
      name = target.getModelName.call();
    }
    const instance = owner.lookup(`service:repository/${name}`);
    return configuration => desc.value.apply(instance, [params, configuration]);
  });
  return desc;
};
export const match = path => {
  return router.match(path);
};
