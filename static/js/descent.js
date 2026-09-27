// Publishes scroll progress (0 → 1) as the --depth custom property so CSS can
// darken the forest backdrop the further down the page you go.

const root = document.documentElement;
const bar = document.querySelector('[data-depth-bar]');
let queued = false;

const update = () => {
  queued = false;
  const max = root.scrollHeight - innerHeight;
  const depth = max > 0 ? Math.min(1, scrollY / max) : 0;
  root.style.setProperty('--depth', depth.toFixed(4));
  if (bar) bar.style.transform = `scaleX(${depth})`;
};

addEventListener('scroll', () => { if (!queued) { queued = true; requestAnimationFrame(update); } }, { passive: true });
addEventListener('resize', update);
update();
