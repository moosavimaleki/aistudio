"use strict";

(() => {
  function samplesFor(image) {
    const samples = [];
    for (let row = 0; row < 10; row++) {
      for (let column = 0; column < 10; column++) {
        const x = Math.min(image.width - 1, Math.floor((column + 0.5) * image.width / 10));
        const y = Math.min(image.height - 1, Math.floor((row + 0.5) * image.height / 10));
        const offset = (y * image.width + x) * 4;
        if (image.data[offset + 3] < 128) continue;
        samples.push({ x, y, red: image.data[offset], green: image.data[offset + 1], blue: image.data[offset + 2] });
      }
    }
    return samples;
  }

  function scoreAt(screen, samples, x, y, scale, quick) {
    let difference = 0;
    let count = 0;
    const increment = quick ? 5 : 1;
    for (let index = 0; index < samples.length; index += increment) {
      const sample = samples[index];
      const offset = ((y + Math.floor(sample.y * scale)) * screen.width + x + Math.floor(sample.x * scale)) * 4;
      difference += Math.abs(screen.data[offset] - sample.red);
      difference += Math.abs(screen.data[offset + 1] - sample.green);
      difference += Math.abs(screen.data[offset + 2] - sample.blue);
      count++;
    }
    return count ? 1 - difference / (count * 3 * 255) : 0;
  }

  function findMatch(screen, reference, threshold = 0.86) {
    if (!screen?.data || !reference?.data || !reference.width || !reference.height) return null;
    const samples = samplesFor(reference);
    if (samples.length < 12) return null;
    let best = null;
    for (const scale of [0.5, 0.75, 1, 1.25, 1.5, 2]) {
      const width = Math.ceil(reference.width * scale);
      const height = Math.ceil(reference.height * scale);
      const maxX = screen.width - width;
      const maxY = screen.height - height;
      if (maxX < 0 || maxY < 0) continue;
      const step = Math.max(2, Math.floor(Math.min(width, height) / 8));
      for (let y = 0; y <= maxY + step; y += step) {
        for (let x = 0; x <= maxX + step; x += step) {
          const left = Math.min(x, maxX);
          const top = Math.min(y, maxY);
          if (scoreAt(screen, samples, left, top, scale, true) < threshold - 0.08) continue;
          const score = scoreAt(screen, samples, left, top, scale, false);
          if (!best || score > best.score) best = { x: left, y: top, width, height, scale, score };
        }
      }
    }
    if (!best || best.score < threshold) return null;
    const step = Math.max(2, Math.floor(Math.min(best.width, best.height) / 8));
    for (let y = Math.max(0, best.y - step); y <= Math.min(screen.height - best.height, best.y + step); y++) {
      for (let x = Math.max(0, best.x - step); x <= Math.min(screen.width - best.width, best.x + step); x++) {
        const score = scoreAt(screen, samples, x, y, best.scale, false);
        if (score > best.score) best = { ...best, x, y, score };
      }
    }
    return best;
  }

  globalThis.AIStudioImageMatch = { findMatch };
  if (typeof module !== "undefined" && module.exports) module.exports = { findMatch };
})();
