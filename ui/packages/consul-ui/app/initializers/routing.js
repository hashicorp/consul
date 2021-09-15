import Route from 'consul-ui/routing/route';

export default {
  name: 'routing',
  initialize(application) {
    application.register('route:basic', Route);
  },
};
