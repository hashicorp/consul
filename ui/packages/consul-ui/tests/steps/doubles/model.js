export default function(scenario, create, win = window, doc = document) {
  scenario
    .given(['an external edit results in $number $model model[s]?'], function(number, model) {
      return create(number, model);
    })
    .given(['$number $model model[s]?'], function(number, model) {
      return create(number, model);
    })
    .given(['$number $model model[s]? with the value "$value"'], function(number, model, value) {
      return create(number, model, value);
    })
    .given(
      ['$number $model model[s]? from yaml\n$yaml', '$number $model model[s]? from json\n$json'],
      function(number, model, data) {
        return create(number, model, data);
      }
    )
    .given(['settings from yaml\n$yaml'], function(data) {
      return Object.keys(data).forEach(function(key) {
        win.localStorage[key] = JSON.stringify(data[key]);
      });
    })
    .given(['ui_config from yaml\n$yaml'], function(data) {
      doc.cookie = `CONSUL_UI_CONFIG=${JSON.stringify(data)}`;
    });
}
