import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default Route.extend({
  model: function() {
    const err = new EmberError('Page not found');
    err.code = '404';
    throw err;
  },
});
