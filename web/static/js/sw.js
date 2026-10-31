const CACHE_VERSION = 2;
const CACHE = "cais-static-v" + CACHE_VERSION;

const PRECACHE = [
  "/static/css/styles.css",
  "/static/js/amarra.js",
  "/static/manifest.webmanifest",
  "/static/icons/icon-192.png",
  "/static/icons/icon-512.png",
  "/static/offline.html",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(PRECACHE))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

// #114: view.Write sends Cache-Control: no-store on authenticated pages, and
// Drive fragments are per-user. The SW used to persist any ok response, so a
// shared device could serve the previous user's dashboard offline.
function putIfCacheable(request, response) {
  const cacheControl = response.headers.get("cache-control") || "";
  if (/no-store|private/i.test(cacheControl)) {
    return Promise.resolve();
  }
  const copy = response.clone();
  return caches.open(CACHE).then((cache) => cache.put(request, copy));
}

// Network-first for HTML Drive, amarra.js, and Tailwind CSS so template
// and runtime updates show without a CACHE_VERSION bump. Other /static/
// assets stay cache-first. Navigations fall back to offline.html.
//
// allowCachedFallback: only the static allowlist (amarra.js/css) may be served
// from the cache; pages and Drive fragments never fall back to cached HTML.
function networkFirst(request, fallbackURL, allowCachedFallback = false) {
  // #359: a bare fetch() can be satisfied by a year-old HTTP cache of the
  // unversioned URL. no-cache revalidates; Cache Storage still covers offline.
  return fetch(request, { cache: "no-cache" })
    .then(async (response) => {
      if (response && response.ok) {
        await putIfCacheable(request, response);
      }
      return response;
    })
    .catch(() => {
      if (allowCachedFallback) {
        return caches.match(request).then((cached) => {
          if (cached) return cached;
          if (!fallbackURL) return Response.error();
          return caches.match(fallbackURL).then((fb) => fb || Response.error());
        });
      }
      if (!fallbackURL) return Response.error();
      return caches.match(fallbackURL).then((fb) => fb || Response.error());
    });
}

function cacheFirst(request) {
  return caches.match(request).then((cached) => {
    if (cached) return cached;
    return fetch(request).then((response) => {
      if (response && response.ok) {
        return putIfCacheable(request, response).then(() => response);
      }
      return response;
    });
  });
}

// Apps may post { type: "amarra:clear-cache" } after a logout.
self.addEventListener("message", (event) => {
  const data = event.data || {};
  if (data.type === "amarra:clear-cache") {
    event.waitUntil(caches.delete(CACHE));
  }
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Logout wipes user-scoped entries even if a page had been cached before the
  // no-store policy existed.
  if (request.method === "POST" && url.pathname === "/logout") {
    event.respondWith(
      fetch(request).then(async (response) => {
        await caches.delete(CACHE);
        return response;
      })
    );
    return;
  }

  if (request.method !== "GET") {
    return;
  }

  if (url.pathname === "/static/js/amarra.js" || url.pathname.startsWith("/static/css/")) {
    event.respondWith(networkFirst(request, null, true));
    return;
  }

  if (url.pathname.startsWith("/static/")) {
    event.respondWith(cacheFirst(request));
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkFirst(request, "/static/offline.html"));
    return;
  }

  if (request.headers.get("accept")?.includes("text/html")) {
    event.respondWith(networkFirst(request));
  }
});
