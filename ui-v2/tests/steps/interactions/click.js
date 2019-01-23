/* eslint no-console: "off" */
export default function(scenario, click, currentPage) {
  scenario
    .when('I click "$selector"', function(selector) {
      return click(selector);
    })
    // TODO: Probably nicer to think of better vocab than having the 'without " rule'
    .when('I click (?!")$property(?!")', function(property) {
      try {
        return currentPage()[property]();
      } catch (e) {
        console.error(e);
        throw new Error(`The '${property}' property on the page object doesn't exist`);
      }
    })
    .when('I click $prop on the $component', function(prop, component) {
      // Collection
      var obj;
      if (typeof currentPage()[component].objectAt === 'function') {
        obj = currentPage()[component].objectAt(0);
      } else {
        obj = currentPage()[component];
      }
      const func = obj[prop].bind(obj);
      try {
        return func();
      } catch (e) {
        throw new Error(
          `The '${prop}' property on the '${component}' page object doesn't exist.\n${e.message}`
        );
      }
    });
}
