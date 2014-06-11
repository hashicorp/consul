Ember.Handlebars.helper('panelBar', function(status) {
  var highlightClass;

  if (status == "passing") {
    highlightClass = "bg-green";
  } else {
    highlightClass = "bg-orange";
  }
  return new Handlebars.SafeString('<div class="panel-bar ' + highlightClass + '"></div>');
});

Ember.Handlebars.helper('listBar', function(status) {
  var highlightClass;

  if (status == "passing") {
    highlightClass = "bg-green";
  } else {
    highlightClass = "bg-orange";
  }
  return new Handlebars.SafeString('<div class="list-bar-horizontal ' + highlightClass + '"></div>');
});

Ember.Handlebars.helper('sessionName', function(session) {
  if (session.Name === "") {
    return session.ID;
  } else {
    return new Handlebars.SafeString(session.Name + ' <small>' + session.ID + '</small>');
  }
});

// We need to do this because of our global namespace properties. The
// service.Tags
Ember.Handlebars.helper('serviceTagMessage', function(tags) {
  if (tags === null) {
    return "No tags";
  }
});
