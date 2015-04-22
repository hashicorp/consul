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
  var name;

  if (session.Name === "") {
    name = '<span>' + session.ID + '</span>';
  } else {
    name = '<span>' + session.Name + '</span>' + ' <small>' + session.ID + '</small>';
  }

  return new Handlebars.SafeString(name);
});

Ember.Handlebars.helper('sessionMeta', function(session) {
  var meta = '<div class="metadata">' + session.Behavior + ' behavior</div>';

  if (session.TTL !== "") {
    meta = meta + '<div class="metadata">, ' + session.TTL + ' TTL</div>';
  }

  return new Handlebars.SafeString(meta);
});

Ember.Handlebars.helper('aclName', function(name, id) {
  if (name === "") {
    return id;
  } else {
    return new Handlebars.SafeString(name + ' <small class="pull-right no-case">' + id + '</small>');
  }
});


Ember.Handlebars.helper('formatRules', function(rules) {
  if (rules === "") {
    return "No rules defined";
  } else {
    return rules;
  }
});


// We need to do this because of our global namespace properties. The
// service.Tags
Ember.Handlebars.helper('serviceTagMessage', function(tags) {
  if (tags === null) {
    return "No tags";
  }
});


// Sends a new notification to the UI
function notify(message, ttl) {
  if (window.notifications !== undefined && window.notifications.length > 0) {
    $(window.notifications).each(function(i, v) {
      v.dismiss();
    });
  }
  var notification = new NotificationFx({
    message : '<p>'+ message + '</p>',
    layout : 'growl',
    effect : 'slide',
    type : 'notice',
    ttl: ttl,
  });

  // show the notification
  notification.show();

  // Add the notification to the queue to be closed
  window.notifications = [];
  window.notifications.push(notification);
}
