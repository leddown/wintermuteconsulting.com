// Wolf eyeshine in the dark. Every 5 to 11 seconds some pairs fade in, linger
// a moment, then fade out; by the next appearance they may be somewhere else,
// or gone. No glitch, no flash: just eyes in the forest. They never track the
// pointer. Under reduced motion they are drawn once, still.
//
//   <canvas data-wolf-eyes data-pairs="3">   up to three pairs, some shown each time
//   data-scale="0.6"                         draw the eyes smaller (or larger)
//   data-band="0.6 0.9"                      keep the eyes inside this band of the
//                                            canvas height (top bottom, 0 to 1), e.g.
//                                            a picture's forest floor: far eyes at
//                                            the top of it, nearer ones lower
//   data-color="ice"                         pale blue instead of amber
//   data-size="lg"                           one large centred pair
//   data-footage="/static/images/opt/eyeshine.webm"
//                                            draw real eyeshine footage (image or
//                                            video, eyes on pure black) instead of
//                                            the vector eyes; falls back to them
//                                            until the file has loaded

const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const rand = (min, max) => min + Math.random() * (max - min);
const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
const rgba = ([r, g, b], a) => `rgba(${r},${g},${b},${a})`;
const FADE_IN = 900, FADE_OUT = 1600, BLINK = 240; // ms

// Eyeshine is a flat, dull reflection: a dim hot centre fading to the edge.
const PALETTES = {
  amber: { hot: [255, 196, 110], mid: [214, 128, 34], glow: [190, 90, 20] },
  ice: { hot: [214, 240, 248], mid: [110, 170, 196], glow: [70, 130, 160] },
};

class Pair {
  constructor(field) {
    this.field = field;
    this.state = 'hidden';
    this.t = Infinity; // time left in the current state
    this.place();
  }

  place() {
    const { w, h, large } = this.field;
    if (large) {
      this.x = w / 2; this.y = h * 0.3; this.s = clamp(w / 28, 14, 30) * this.field.scale;
      return;
    }
    // Deeper in the forest = higher up and smaller.
    // On narrow screens the copy fills the lower half, so keep eyes above it,
    // unless the canvas names the band of its picture they belong in.
    const [top, bottom] = this.field.band || (w < 700 ? [0.1, 0.28] : [0.42, 0.72]);
    // Two pairs side by side read as one animal with four eyes, which a narrow
    // band makes likely: look a few times for a spot clear of the others.
    for (let tries = 0; tries < 16; tries++) {
      const depth = Math.random();
      this.x = rand(w * 0.12, w * 0.88);
      this.s = (4 + depth * 6 + (w > 1200 ? 2 : 0)) * this.field.scale;
      // In a named band the whole eye stays inside it, not just its centre.
      const inset = this.field.band ? this.s * 0.6 : 0;
      this.y = clamp(h * (top + depth * (bottom - top)), h * top + inset, h * bottom - inset);
      if (!this.crowded()) break;
    }
  }

  // Close enough to another pair that the two would read as one cluster. The
  // eyes of a pair sit about 4 sizes apart, so pairs at the same height need
  // well over that between them or they read as four eyes in a row.
  crowded() {
    return (this.field.pairs || []).some(o => o !== this &&
      Math.abs(o.x - this.x) < (o.s + this.s) * 8 && Math.abs(o.y - this.y) < (o.s + this.s) * 1.5);
  }

  // Chosen this time: fade in (or stay) and linger a little.
  reveal() {
    if (this.state === 'hidden' || this.state === 'out') this.place();
    this.state = 'in';
    this.t = FADE_IN;
    this.linger = rand(1200, 2800);
  }

  // Not chosen this time: whatever was there slips away.
  vanish() {
    if (this.state === 'hidden' || this.state === 'out') return;
    this.state = 'out';
    this.t = FADE_OUT * 0.5;
  }

  update(dt) {
    this.blinkIn = (this.blinkIn ?? rand(1500, 6000)) - dt;
    if (this.blinkIn < 0) { this.blinkT = BLINK; this.blinkIn = rand(7000, 15000); }
    if (this.blinkT > 0) this.blinkT -= dt;
    this.phase = (this.phase ?? rand(0, 6)) + dt / 1700;
    this.flicker = 0.82 + 0.18 * Math.sin(this.phase);

    this.t -= dt;
    if (this.t > 0) return;
    switch (this.state) {
      case 'in': this.state = 'open'; this.t = this.linger; break;
      case 'open': this.state = 'out'; this.t = FADE_OUT; break;
      case 'out': this.state = 'hidden'; this.t = Infinity; break;
    }
  }

  get alpha() {
    if (this.state === 'in') return clamp(1 - this.t / FADE_IN, 0, 1);
    if (this.state === 'open') return 1;
    if (this.state === 'out') return clamp(this.t / FADE_OUT, 0, 1);
    return 0;
  }

  get openness() {
    if (!(this.blinkT > 0)) return 1;
    const p = this.blinkT / BLINK; // 1 → 0
    return Math.abs(p * 2 - 1);  // close then reopen
  }
}

class Field {
  constructor(canvas) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.large = canvas.dataset.size === 'lg';
    this.palette = PALETTES[canvas.dataset.color] || PALETTES.amber;
    this.scale = +canvas.dataset.scale || 1;
    const band = (canvas.dataset.band || '').trim().split(/\s+/).map(Number);
    this.band = band.length === 2 && band.every(n => n >= 0 && n <= 1) && band[0] < band[1] ? band : null;
    this.visible = false;
    this.resize();
    this.pairs = Array.from({ length: this.large ? 1 : +canvas.dataset.pairs || 3 }, () => new Pair(this));
    if (canvas.dataset.footage) this.loadFootage(canvas.dataset.footage);
    new ResizeObserver(() => { this.resize(); this.pairs.forEach(p => p.place()); if (reduced) this.drawStatic(); }).observe(canvas);
    new IntersectionObserver(([e]) => { this.visible = e.isIntersecting; }).observe(canvas);
  }

  // Footage is eyes on black. Blending the whole canvas with 'screen' makes the
  // black drop out against the page behind it.
  useFootage(f) {
    this.footage = f;
    this.canvas.style.mixBlendMode = 'screen';
    if (reduced) this.drawStatic();
  }

  loadFootage(src) {
    if (/\.(webm|mp4)$/i.test(src)) {
      const v = document.createElement('video');
      Object.assign(v, { muted: true, loop: true, playsInline: true, preload: 'auto', src });
      v.addEventListener('loadeddata', () => this.useFootage(v), { once: true });
      return;
    }
    const img = new Image();
    img.decoding = 'async';
    img.onload = () => this.useFootage(img);
    img.src = src;
  }

  // Called on each appearance while the canvas is on screen.
  appear() {
    const shown = this.large ? this.pairs : this.pairs.filter(() => Math.random() < 0.55);
    if (!shown.length) shown.push(this.pairs[(Math.random() * this.pairs.length) | 0]);
    for (const p of this.pairs) shown.includes(p) ? p.reveal() : p.vanish();
    if (this.footage instanceof HTMLVideoElement) this.footage.play().catch(() => {});
  }

  resize() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    const r = this.canvas.getBoundingClientRect();
    this.w = r.width; this.h = r.height;
    this.canvas.width = Math.round(r.width * dpr);
    this.canvas.height = Math.round(r.height * dpr);
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  // An almond-shaped reflection, inner corners low for a narrowed stare. No iris, no pupil.
  drawEye(cx, cy, s, tilt, open, alpha) {
    const { ctx, palette } = this;

    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(tilt);
    ctx.scale(1, Math.max(open, 0.02));
    const h = s * 0.34;
    const fill = ctx.createRadialGradient(0, 0, 0, 0, 0, s);
    fill.addColorStop(0, rgba(palette.hot, 0.9 * alpha));
    fill.addColorStop(0.3, rgba(palette.mid, 0.75 * alpha));
    fill.addColorStop(1, rgba(palette.glow, 0.05 * alpha));
    ctx.fillStyle = fill;
    ctx.beginPath();
    ctx.moveTo(-s, 0);
    ctx.quadraticCurveTo(0, -h * 1.9, s, 0);
    ctx.quadraticCurveTo(0, h * 1.5, -s, 0);
    ctx.fill();
    ctx.restore();
  }

  drawPair(p) {
    const a = p.alpha * (p.flicker ?? 1);
    if (a <= 0) return;
    const gap = p.s * 2.1;
    if (this.footage) {
      this.drawFootage(p, gap, a);
      return;
    }
    // One faint shared glow on the fur around both eyes.
    const { ctx, palette } = this;
    ctx.save();
    ctx.translate(p.x, p.y);
    ctx.scale(1, 0.55);
    const halo = ctx.createRadialGradient(0, 0, 0, 0, 0, gap * 2.2);
    halo.addColorStop(0, rgba(palette.glow, 0.1 * a * p.openness));
    halo.addColorStop(1, rgba(palette.glow, 0));
    ctx.fillStyle = halo;
    ctx.beginPath(); ctx.arc(0, 0, gap * 2.2, 0, Math.PI * 2); ctx.fill();
    ctx.restore();
    this.drawEye(p.x - gap, p.y, p.s, 0.22, p.openness, a);
    this.drawEye(p.x + gap, p.y, p.s, -0.22, p.openness, a);
  }

  // Real footage. Blinks are left to the footage itself when it is a video.
  drawFootage(p, gap, a) {
    const { ctx, footage: f } = this;
    const fw = f.videoWidth || f.naturalWidth, fh = f.videoHeight || f.naturalHeight;
    if (!fw || !fh) return;
    const w = (gap + p.s) * 2 * 1.4, h = w * fh / fw;
    const squash = f instanceof HTMLVideoElement ? 1 : Math.max(p.openness, 0.02);
    ctx.save();
    ctx.globalAlpha = a;
    ctx.translate(p.x, p.y);
    ctx.scale(1, squash);
    ctx.drawImage(f, -w / 2, -h / 2, w, h);
    ctx.restore();
  }

  frame(dt) {
    if (!this.visible) return;
    this.ctx.clearRect(0, 0, this.w, this.h);
    for (const p of this.pairs) { p.update(dt); this.drawPair(p); }
    const v = this.footage;
    if (v instanceof HTMLVideoElement && !v.paused && this.pairs.every(p => p.state === 'hidden')) v.pause();
  }

  drawStatic() {
    this.ctx.clearRect(0, 0, this.w, this.h);
    for (const p of this.pairs) { p.state = 'open'; this.drawPair(p); }
  }
}

const fields = [...document.querySelectorAll('canvas[data-wolf-eyes]')].map(c => new Field(c));

if (reduced) {
  fields.forEach(f => f.drawStatic());
} else if (fields.length) {
  const appear = () => {
    if (!document.hidden) fields.forEach(f => f.visible && f.appear());
    setTimeout(appear, rand(5000, 11000));
  };
  setTimeout(appear, 1400);
  let last = performance.now();
  const loop = now => {
    const dt = Math.min(now - last, 100);
    last = now;
    fields.forEach(f => f.frame(dt));
    requestAnimationFrame(loop);
  };
  requestAnimationFrame(loop);
}
