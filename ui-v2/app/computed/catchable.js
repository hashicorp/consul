import ComputedProperty from '@ember/object/computed';
import computedFactory from 'consul-ui/utils/computed/factory';

export default class Catchable extends ComputedProperty {
  catch(cb) {
    return this.meta({
      catch: cb,
    });
  }
}
export const computed = computedFactory(Catchable);
