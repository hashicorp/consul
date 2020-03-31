import Service from '@ember/service';
export default Service.extend({
  matches: function(state, matches) {
    const values = Array.isArray(matches) ? matches : [matches];
    return values.some(item => {
      return state.matches(item);
    });
  },
  state: function(cb) {
    return {
      matches: cb,
    };
  },
});
