export default function(scenario, pages, set) {
  scenario
    .when('I visit the $name page', function(name) {
      return set(pages[name]).visit();
    })
    .when('I visit the $name page for the "$id" $model', function(name, id, model) {
      return set(pages[name]).visit({
        [model]: id,
      });
    })
    .when(
      ['I visit the $name page for yaml\n$yaml', 'I visit the $name page for json\n$json'],
      function(name, data) {
        // TODO: Consider putting an assertion here for testing the current url
        // do I absolutely definitely need that all the time?
        return set(pages[name]).visit(data);
      }
    );
}
