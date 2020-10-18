import Component from '@ember/component';

export default Component.extend({
  tagName: '',
  didInsertElement: function() {
    this._super(...arguments);
    this.chart.addAction(this.name, (context, event) => this.exec(context, event));
  },
  willDestroy: function() {
    this._super(...arguments);
    this.chart.removeAction(this.type);
  },
});
