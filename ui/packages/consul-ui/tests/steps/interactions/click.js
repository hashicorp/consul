export default function(scenario, find, click) {
  scenario
    .when('I click "$selector"', function(selector) {
      return click(selector);
    })
    // TODO: Probably nicer to think of better vocab than having the 'without " rule'
    .when(
      [
        'I click (?!")$property(?!")',
        'I click $property on the $component',
        'I click $property on the $component component',
      ],
      function(property, component, next) {
        if (typeof component === 'string') {
          property = `${component}.${property}`;
        }
        return find(property)();
      }
    );
}
