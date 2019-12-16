export default function(scenario, pauseUntil, find, click) {
  scenario
    .when('I click "$selector"', function(selector) {
      return pauseUntil(function(resolve, reject) {
        const $el = document.querySelector(selector);
        if ($el) {
          return click(selector).then(resolve);
        }
        return Promise.resolve();
      });
    })
    // TODO: Probably nicer to think of better vocab than having the 'without " rule'
    .when(['I click (?!")$property(?!")', 'I click $property on the $component'], function(
      property,
      component
    ) {
      let prop = property;
      if (typeof component === 'string') {
        prop = `${component}.${property}`;
      }
      return pauseUntil(function(resolve, reject) {
        try {
          return find(prop)().then(resolve);
        } catch (e) {
          return Promise.resolve();
        }
      });
    });
}
