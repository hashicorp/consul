import RSVP, { Promise } from 'rsvp';
export default function(EventTarget = RSVP.EventTarget, P = Promise) {
  // TODO: Class-ify
  return function(filter) {
    return EventTarget.mixin({
      value: '',
      add: function(data) {
        this.data = data;
        return this;
      },
      search: function(term = '') {
        this.value = term === null ? '' : term.trim();
        // specifically no return here we return `this` instead
        // right now filtering is sync but we introduce an async
        // flow now for later on
        P.resolve(
          this.value !== ''
            ? this.data.filter(item => {
                return filter(item, { s: term });
              })
            : this.data
        ).then(data => {
          // TODO: For the moment, lets just fake a target
          this.trigger('change', {
            target: {
              value: this.value,
              // TODO: selectedOptions is what <select> uses, consider that
              data: data,
            },
          });
          // not returned
          return data;
        });
        return this;
      },
    });
  };
}
