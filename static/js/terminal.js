// Typed "AI reasoning feed" for [data-terminal], plus a live UTC clock for [data-clock].

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const wait = ms => new Promise(r => setTimeout(r, ms));

for (const term of document.querySelectorAll('[data-terminal]')) {
  const out = term.querySelector('.terminal__out');
  const lines = [...term.querySelector('template[data-lines]').content.children];

  if (reduced) {
    out.append(...lines.map(l => l.cloneNode(true)));
    continue;
  }

  let started = false;
  const run = async () => {
    for (;;) {
      out.replaceChildren();
      for (const src of lines) {
        const li = src.cloneNode(false);
        li.classList.add('is-typing');
        out.append(li);
        const text = src.textContent;
        for (let i = 1; i <= text.length; i++) {
          li.textContent = text.slice(0, i);
          await wait(text[i - 1] === ' ' ? 8 : 18 + Math.random() * 22);
        }
        li.classList.remove('is-typing');
        await wait(src.classList.contains('crit') ? 1400 : 350 + Math.random() * 500);
      }
      await wait(6000);
    }
  };

  new IntersectionObserver(([e], obs) => {
    if (e.isIntersecting && !started) { started = true; obs.disconnect(); run(); }
  }, { threshold: 0.3 }).observe(term);
}

for (const clock of document.querySelectorAll('[data-clock]')) {
  const tick = () => { clock.textContent = new Date().toISOString().slice(11, 19) + ' UTC'; };
  tick();
  setInterval(tick, 1000);
}
