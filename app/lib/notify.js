// quick console notify for the moment
export default function(message) {
  console.log(message);
}

// export default function(Notification)
// {
//     // Sends a new notification to the UI
//     return function(message, ttl) {
//         if (window.notifications !== undefined && window.notifications.length > 0) {
//             window.notifications.forEach(function(item, i) {
//                 item.dismiss();
//             });
//         }
//         var notification = new Notification({
//             message : '<p>'+ message + '</p>',
//             layout : 'growl',
//             effect : 'slide',
//             type : 'notice',
//             ttl: ttl
//         });

//         // show the notification
//         notification.show();

//         // Add the notification to the queue to be closed
//         window.notifications = [];
//         window.notifications.push(notification);
//     }

// }
