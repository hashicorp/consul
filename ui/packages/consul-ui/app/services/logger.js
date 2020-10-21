import Service from '@ember/service';
import { runInDebug } from '@ember/debug';

export default Service.extend({
  execute: function(obj) {
    runInDebug(() => {
      obj = typeof obj.error !== 'undefined' ? obj.error : obj;
      if (obj instanceof Error) {
        console.error(obj); // eslint-disable-line no-console
      } else {
        console.log(obj); // eslint-disable-line no-console
      }
    });
  },
});
