import Route from '@ember/routing/route';
import Error from '@ember/error';

export default Route.extend({
  beforeModel: function() {
    const e = new Error('Page not found');
    e.code = 404;
    throw e;
  },
});
