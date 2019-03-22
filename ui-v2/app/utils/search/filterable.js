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
      find: function(terms = []) {
        this.value = terms
          .filter(function(item) {
            return typeof item === 'string' && item !== '';
          })
          .map(function(term) {
            return term.trim();
          });
        return P.resolve(
          this.value.reduce(function(prev, term) {
            return prev.filter(item => {
              return filter(item, { s: term });
            });
          }, this.data)
        );
      },
      search: function(terms = []) {
        // specifically no return here we return `this` instead
        // right now filtering is sync but we introduce an async
        // flow now for later on
        this.find(Array.isArray(terms) ? terms : [terms]).then(data => {
          // TODO: For the moment, lets just fake a target
          this.trigger('change', {
            target: {
              value: this.value.join('\n'),
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
