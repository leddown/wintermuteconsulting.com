// Send the contact form ([data-contact]) without leaving the page. The server's
// answer, the thank-you notice or the form again with its errors, replaces the
// form where it stands, so the visitor sees it without the page reloading and
// scrolling down from the top. Without this script the form posts and
// redirects as before.

const show = el => {
  el.focus({ preventScroll: true });
  const r = el.getBoundingClientRect();
  if (r.top < 80 || r.bottom > innerHeight) el.scrollIntoView({ block: 'center', behavior: 'instant' });
};

const enhance = form => form.addEventListener('submit', async e => {
  e.preventDefault();
  if (form.getAttribute('aria-busy') === 'true') return;
  form.setAttribute('aria-busy', 'true');
  const button = form.querySelector('[type=submit]');
  if (button) button.disabled = true;

  let next;
  try {
    const res = await fetch(form.action, { method: 'POST', body: new URLSearchParams(new FormData(form)) });
    const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
    next = doc.querySelector('.form-status--ok, form[data-contact]');
  } catch { /* network trouble: fall through to a normal post */ }
  // Anything unexpected (a plain error page, no connection): let the browser
  // post the form itself and show whatever comes back.
  if (!next) { form.submit(); return; }

  next = document.importNode(next, true);
  form.replaceWith(next);
  if (next.matches('form')) {
    enhance(next);
    show(next.querySelector('[aria-invalid="true"], .form-status--err') || next);
  } else {
    show(next);
  }
});

document.querySelectorAll('form[data-contact]').forEach(enhance);
