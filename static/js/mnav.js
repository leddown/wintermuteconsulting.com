// Mobile menu for <details data-menu>. Works without JS (tap to open/close);
// this adds closing on link tap, Escape and outside tap, and locks page scroll
// while the sheet is open.

for (const menu of document.querySelectorAll('details[data-menu]')) {
  const close = () => { menu.open = false; };
  menu.addEventListener('toggle', () => {
    document.documentElement.classList.toggle('m-menu-open', menu.open);
  });
  menu.querySelectorAll('a').forEach(a => a.addEventListener('click', close));
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && menu.open) { close(); menu.querySelector('summary').focus(); }
  });
  document.addEventListener('click', e => {
    if (menu.open && !menu.contains(e.target)) close();
  });
}
