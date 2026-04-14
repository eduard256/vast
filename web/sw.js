// VAST Service Worker -- push notifications only, no offline cache

self.addEventListener('install', function() {
  self.skipWaiting();
});

self.addEventListener('activate', function(e) {
  e.waitUntil(self.clients.claim());
});

// Handle push notifications
self.addEventListener('push', function(e) {
  var data = { title: 'VAST', body: 'Уведомление' };
  if (e.data) {
    try {
      data = e.data.json();
    } catch (_) {
      data.body = e.data.text();
    }
  }

  e.waitUntil(
    self.registration.showNotification(data.title || 'VAST', {
      body: data.body || '',
      icon: '/icon-192.svg',
      badge: '/icon-192.svg',
      tag: data.tag || 'vast-notification',
      data: data.data || {},
      vibrate: [200, 100, 200]
    })
  );
});

// Handle notification click -- open or focus the app
self.addEventListener('notificationclick', function(e) {
  e.notification.close();
  var url = '/';
  if (e.notification.data && e.notification.data.url) {
    url = e.notification.data.url;
  }
  e.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function(clients) {
      for (var i = 0; i < clients.length; i++) {
        if (clients[i].url.includes(self.location.origin)) {
          clients[i].navigate(url);
          return clients[i].focus();
        }
      }
      return self.clients.openWindow(url);
    })
  );
});
