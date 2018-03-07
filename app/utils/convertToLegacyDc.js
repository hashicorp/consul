import { assign } from '@ember/polyfills';
// this is a temporary function to convert dc objects
// back to simple strings
export default function(prop) {
  return function(model) {
    return assign({}, model, {
      [prop]: model[prop].map(function(item) {
        return item.get('Name');
      }),
    });
  };
}
