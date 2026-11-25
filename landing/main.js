/* Kinkit landing page: small, dependency-free enhancements.
 *  - points every "Start free" / "Sign in" link at the app (APP_URL below)
 *  - mobile menu, sticky-header shadow, scroll reveal, Kiki in-view motion, one-open-at-a-time FAQ
 *  - nothing language-specific: the page is English only, like the app UI
 * The page works fully without JavaScript; this only adds polish.
 */
(function () {
  'use strict';

  // CONFIG: the only thing you normally need to change.
  // Base URL of the Kinkit web app. '' means "same origin as this page"
  // (e.g. landing served at / and the app on the same host).
  // Examples: 'https://app.kinkit.app'  or  'https://kinkit.example.com'
  var APP_URL = '';
  // A deploy can set it without editing this file: <meta name="kinkit-app-url" content="https://...">
  var appMeta = document.querySelector('meta[name="kinkit-app-url"]');
  if (appMeta && appMeta.content) APP_URL = appMeta.content;

  // Paths inside the app. The React app shows its sign-up form on /login
  // (toggle "Sign up"); point SIGNUP at '/login' if no /signup route exists.
  var APP_PATHS = { signup: '/login', login: '/login', docs: '/docs' }; // docs: the in-app documentation site; with no app URL set it stays a relative /docs

  var root = document.documentElement;

  // app links
  var base = APP_URL.replace(/\/+$/, '');
  document.querySelectorAll('[data-app-link]').forEach(function (a) {
    var path = APP_PATHS[a.getAttribute('data-app-link')];
    if (path) a.setAttribute('href', base + path);
  });

  // download sizes (HEAD request; hidden when the file is missing)
  document.querySelectorAll('[data-dl-size]').forEach(function (el) {
    if (!window.fetch) return;
    fetch(el.getAttribute('data-dl-size'), { method: 'HEAD' }).then(function (r) {
      var n = r.ok ? Number(r.headers.get('content-length')) : 0;
      if (n > 0) el.textContent = n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB';
    }).catch(function () {});
  });

  // footer year
  var year = document.getElementById('year');
  if (year) year.textContent = String(new Date().getFullYear());

  // mobile menu
  var menuBtn = document.querySelector('.menu-btn');
  var navLinks = document.getElementById('nav-links');
  function setMenu(open) {
    if (!menuBtn || !navLinks) return;
    menuBtn.setAttribute('aria-expanded', String(open));
    navLinks.classList.toggle('open', open);
    menuBtn.setAttribute('aria-label', open ? 'Close menu' : 'Open menu');
  }
  if (menuBtn && navLinks) {
    menuBtn.addEventListener('click', function () {
      setMenu(menuBtn.getAttribute('aria-expanded') !== 'true');
    });
    navLinks.addEventListener('click', function (e) {
      if (e.target.closest('a')) setMenu(false);
    });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && menuBtn.getAttribute('aria-expanded') === 'true') {
        setMenu(false);
        menuBtn.focus();
      }
    });
    window.matchMedia('(min-width: 961px)').addEventListener('change', function (mq) {
      if (mq.matches) setMenu(false);
    });
  }

  // header shadow once scrolled
  var header = document.querySelector('.site-header');
  function onScroll() {
    if (header) header.classList.toggle('scrolled', window.scrollY > 8);
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  // scroll reveal
  var reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var revealEls = document.querySelectorAll('.reveal');
  if (reduceMotion || !('IntersectionObserver' in window)) {
    revealEls.forEach(function (el) { el.classList.add('in'); });
  } else {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('in');
          io.unobserve(entry.target);
        }
      });
    }, { rootMargin: '0px 0px -8% 0px', threshold: 0.08 });
    revealEls.forEach(function (el) { io.observe(el); });
  }

  // Kiki: start the celebrate / relax motion when it scrolls into view
  // (CSS only animates under prefers-reduced-motion: no-preference, so this is safe either way)
  var mascots = document.querySelectorAll('.mk[data-mk-animate]');
  if (!reduceMotion && 'IntersectionObserver' in window) {
    var mio = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('mk--animate');
          mio.unobserve(entry.target);
        }
      });
    }, { threshold: 0.4 });
    mascots.forEach(function (el) { mio.observe(el); });
  }

  // FAQ: keep one answer open at a time
  var faqItems = document.querySelectorAll('.faq details');
  faqItems.forEach(function (d) {
    d.addEventListener('toggle', function () {
      if (!d.open) return;
      faqItems.forEach(function (other) { if (other !== d) other.open = false; });
    });
  });
})();
