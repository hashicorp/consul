import { get, set } from '@ember/object';
import MultiMap from 'mnemonist/multi-map';

/**
 * Checks are ember-data-model-fragments, so we can't just
 * concat it, we have to loop through all the items in order to merge
 * We also need to avoid repeating Node checks here as the service and the
 * proxy is likely to be on the same node, without adding something extra here
 * the node check will likely end up in the list twice.
 *
 * @param {Array} checks - Multiple lists of healthchecks to merge each one of the items in this array should be a further array of healthchecks
 * @param {Boolean} exposed - Whether the checks should be marked as exposed via the proxy or not
 * @param {Object} MMap - A MultiMap class. This is only exposed to allow for an easier interface but still allow an injectable MultiMap if we choose to do that during testing
 * @returns {Array} - The final array of all of the healthchecks with any duplicate node checks removed, and also marked as exposed if required
 */
export default (checks = [], exposed = false, MMap = MultiMap) => {
  const ids = new MMap();
  const a = checks.shift();
  const result = a
    .map((item) => {
      // its a Node check (ServiceName === ""), record this one so we
      // don't end up with duplicates of it
      if (item.ServiceName === '') {
        ids.set(item.Node, item.CheckID);
      }
      return item;
    })
    // go through all remaining lists of checks adding each check to the
    // list if its not a node check that has been already added
    .concat(
      checks.reduce((prev, items) => {
        if (typeof items === 'undefined') {
          return prev;
        }
        return prev.concat(
          items.reduce((prev, item) => {
            if (item.ServiceName === '') {
              if ((ids.get(item.Node) || []).includes(item.CheckID)) {
                return prev;
              }
              // if the node check hasn't been added yet, record this one
              // so we don't end up with duplicates of it
              ids.set(item.Node, item.CheckID);
            }
            prev.push(item);
            return prev;
          }, [])
        );
      }, [])
    );
  // if checks are exposed via the proxy, find the ones that are exposable
  // (ones of a certain type) and set them as exposed
  // TODO: consider moving this out of here so we aren't doing too much in one util
  if (exposed) {
    result
      .filter((item) => get(item, 'Exposable'))
      .forEach((item) => {
        set(item, 'Exposed', exposed);
      });
  }
  return result;
};
