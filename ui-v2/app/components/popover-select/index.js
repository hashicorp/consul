import Component from '@ember/component';

export default Component.extend({
  actions: {
    change: function(option, e) {
      // We fake an event here, which could be a bit of a footbun if we treat
      // it completely like an event, we should be abe to avoid doing this
      // when we move to glimmer components (this.args.selected vs this.selected)
      this.onchange({
        target: {
          selected: option,
        },
        // make this vaguely event like to avoid
        // having a separate property
        preventDefault: function(e) {},
      });
    },
  },
});
