// Digital glitch effects shared by every page.
//
//   [data-glitch]        text decodes on load and stutters during flashes
//   [data-glitch-hover]  text scrambles briefly on hover/focus
//   [data-flash]         full-screen overlay: a brief dim with a few hairline tears
//
// Flashes are at most two pulses every several seconds, well under the WCAG
// three-flashes-per-second limit, and everything is off under reduced motion.

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
// Latin-range glyphs only: anything outside the self-hosted fonts falls back to
// a wider system font and reflows the headline.
const GLYPHS = 'ABCDEFGHKMNRSTVWXYZ0123456789#/\\<>*+=_ØÆ';

const rand = (min, max) => min + Math.random() * (max - min);
const glyph = () => GLYPHS[(Math.random() * GLYPHS.length) | 0];

function originalText(el) {
  if (!el.dataset.text) el.dataset.text = el.textContent;
  return el.dataset.text;
}

// Replace a share of characters with glyphs for a few frames, then restore.
function scramble(el, { ratio = 0.35, frames = 6, frameMs = 45 } = {}) {
  if (el._scrambling) return;
  el._scrambling = true;
  const text = originalText(el);
  const lockWidth = getComputedStyle(el).display !== 'inline';
  if (lockWidth) el.style.minWidth = `${el.offsetWidth}px`;
  let n = 0;
  const tick = () => {
    if (n++ >= frames) {
      el.textContent = text;
      if (lockWidth) el.style.minWidth = '';
      el._scrambling = false;
      return;
    }
    el.textContent = [...text].map(c => (c.trim() && Math.random() < ratio ? glyph() : c)).join('');
    setTimeout(tick, frameMs);
  };
  tick();
}

// Characters resolve left to right out of noise.
function decode(el, duration = 1100) {
  const text = originalText(el);
  const chars = [...text];
  const start = performance.now();
  const step = now => {
    const p = Math.min(1, (now - start) / duration);
    const settled = Math.floor(p * chars.length);
    el.textContent = chars.map((c, i) => (i < settled || !c.trim() ? c : glyph())).join('');
    if (p < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

function burst(el) {
  el.classList.add('is-glitching');
  scramble(el, { ratio: 0.25, frames: 5 });
  setTimeout(() => el.classList.remove('is-glitching'), 320);
}

function tearBands(layer) {
  layer.replaceChildren();
  const count = (rand(2, 5)) | 0;
  for (let i = 0; i < count; i++) {
    const band = document.createElement('span');
    // Mostly black tears; the occasional faint pale scanline.
    band.className = `flash-band flash-band--${Math.random() < 0.75 ? 'dark' : 'pale'}`;
    band.style.top = `${rand(0, 98)}%`;
    band.style.height = `${rand(1, 3) | 0}px`;
    band.style.transform = `translateX(${rand(-3, 3)}%)`;
    layer.append(band);
  }
}

function flash() {
  const layer = document.querySelector('[data-flash]');
  const root = document.documentElement;
  const pulse = (ms) => {
    if (layer) tearBands(layer);
    root.classList.add('is-flashing');
    setTimeout(() => root.classList.remove('is-flashing'), ms);
  };
  pulse(110);
  // wolfeyes.js reveals the eyes on this: they are only seen when the lights dip.
  document.dispatchEvent(new CustomEvent('wintermute-consulting:flash'));
  if (Math.random() < 0.5) setTimeout(() => pulse(70), 220);

  document.querySelectorAll('[data-glitch]').forEach(el => {
    const r = el.getBoundingClientRect();
    if (r.bottom > 0 && r.top < innerHeight) burst(el);
  });
}

function schedule() {
  setTimeout(() => {
    if (!document.hidden) flash();
    schedule();
  }, rand(5000, 11000));
}

if (!reduced) {
  document.querySelectorAll('[data-glitch]').forEach(el => decode(el));
  setTimeout(flash, 1400);
  schedule();

  const onHover = e => {
    const host = e.target.closest?.('[data-glitch-hover]');
    if (!host) return;
    const target = host.querySelector('[data-glitch]') || (host.children.length === 0 ? host : null);
    if (!target) return;
    if (target.hasAttribute('data-glitch')) burst(target);
    else scramble(target);
  };
  document.addEventListener('pointerover', onHover);
  document.addEventListener('focusin', onHover);
}
