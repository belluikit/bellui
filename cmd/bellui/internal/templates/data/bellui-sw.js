// Bellui notification service worker (pre-rebrand tracker #724).
//
// The page-context Notification constructor cannot render action buttons, and
// action clicks are delivered to a service worker's notificationclick event, not
// to the page. This minimal worker lets Bellui's wasm runtime render rich
// notifications (via registration.showNotification) and routes both the body
// click and each action button back to the page, where Bellui maps them to the
// NotificationOptions.OnTapped / NotificationAction callbacks.
//
// Bellui registers this file lazily and only for notifications that carry actions;
// apps without it degrade to the basic Notification path (body click only).

self.addEventListener('install', function () {
	self.skipWaiting();
});

self.addEventListener('activate', function (event) {
	event.waitUntil(self.clients.claim());
});

self.addEventListener('notificationclick', function (event) {
	event.notification.close();

	var data = event.notification.data || {};
	var message = {
		belluiNotify: true,
		token: data.token,
		action: event.action || '' // '' == body click
	};

	event.waitUntil(
		self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function (clientList) {
			for (var i = 0; i < clientList.length; i++) {
				clientList[i].postMessage(message);
			}
			// Bring the app to the foreground on a body click, matching the
			// desktop/mobile "click to focus" behaviour.
			if (!event.action && clientList.length > 0 && 'focus' in clientList[0]) {
				return clientList[0].focus();
			}
		})
	);
});
