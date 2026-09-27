// Drifting snow on <canvas data-snow>. Three depth layers; the pointer nudges the wind.

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const rand = (min, max) => min + Math.random() * (max - min);

for (const canvas of document.querySelectorAll('canvas[data-snow]')) {
  const ctx = canvas.getContext('2d');
  let w = 0, h = 0, flakes = [], visible = true, wind = 0, targetWind = 0;

  const makeFlake = (anyY) => {
    const z = Math.random(); // 0 far … 1 near
    return {
      x: rand(0, w), y: anyY ? rand(0, h) : rand(-20, -2),
      r: 0.6 + z * 2.4, vy: 0.25 + z * 1.1, sway: rand(0, Math.PI * 2),
      a: 0.35 + z * 0.55,
    };
  };

  const resize = () => {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    const r = canvas.getBoundingClientRect();
    w = r.width; h = r.height;
    canvas.width = Math.round(w * dpr); canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const count = Math.min(260, Math.round((w * h) / 7000));
    flakes = Array.from({ length: count }, () => makeFlake(true));
    if (reduced) draw();
  };

  const draw = () => {
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = '#fff';
    for (const f of flakes) {
      ctx.globalAlpha = f.a;
      ctx.beginPath(); ctx.arc(f.x, f.y, f.r, 0, Math.PI * 2); ctx.fill();
    }
    ctx.globalAlpha = 1;
  };

  const step = () => {
    if (visible) {
      wind += (targetWind - wind) * 0.02;
      for (let i = 0; i < flakes.length; i++) {
        const f = flakes[i];
        f.sway += 0.01;
        f.y += f.vy;
        f.x += Math.sin(f.sway) * 0.3 + wind * f.vy;
        if (f.y > h + 5 || f.x < -10 || f.x > w + 10) flakes[i] = makeFlake(false);
      }
      draw();
    }
    requestAnimationFrame(step);
  };

  new ResizeObserver(resize).observe(canvas);
  new IntersectionObserver(([e]) => { visible = e.isIntersecting; }).observe(canvas);
  addEventListener('pointermove', e => { targetWind = (e.clientX / innerWidth - 0.5) * 1.6; }, { passive: true });

  resize();
  if (!reduced) requestAnimationFrame(step);
}
