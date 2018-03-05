import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  Name(i) {
    return `Datacenter ${i}`;
  },
});
