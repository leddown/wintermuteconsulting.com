// Reveal [data-reveal] elements as they scroll into view, and count up [data-count] numbers.

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;

const countUp = el => {
  const target = +el.dataset.count;
  if (reduced || !target) return;
  const start = performance.now(), dur = 1400;
  const step = now => {
    const p = Math.min(1, (now - start) / dur);
    el.textContent = Math.round(target * (1 - Math.pow(1 - p, 3)));
    if (p < 1) requestAnimationFrame(step);
  };
  el.textContent = '0';
  requestAnimationFrame(step);
};

const io = new IntersectionObserver(entries => {
  for (const e of entries) {
    if (!e.isIntersecting) continue;
    e.target.classList.add('is-visible');
    e.target.querySelectorAll('[data-count]').forEach(countUp);
    io.unobserve(e.target);
  }
}, { rootMargin: '0px 0px -10% 0px' });

document.querySelectorAll('[data-reveal]').forEach(el => io.observe(el));
