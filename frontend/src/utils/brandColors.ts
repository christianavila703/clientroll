type ExtractedBrandColors = {
  primary: string;
  secondary: string;
};

type ColorBucket = {
  rSum: number;
  gSum: number;
  bSum: number;
  count: number;
};

const QUANT_STEP = 24;

function clamp8(value: number): number {
  return Math.max(0, Math.min(255, Math.round(value)));
}

function toHex(value: number): string {
  return clamp8(value).toString(16).padStart(2, "0");
}

function rgbToHex(r: number, g: number, b: number): string {
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

function rgbToHsl(r: number, g: number, b: number): { h: number; s: number; l: number } {
  const rn = r / 255;
  const gn = g / 255;
  const bn = b / 255;
  const max = Math.max(rn, gn, bn);
  const min = Math.min(rn, gn, bn);
  let h = 0;
  const l = (max + min) / 2;
  let s = 0;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case rn:
        h = (gn - bn) / d + (gn < bn ? 6 : 0);
        break;
      case gn:
        h = (bn - rn) / d + 2;
        break;
      default:
        h = (rn - gn) / d + 4;
        break;
    }
    h /= 6;
  }

  return { h, s, l };
}

function colorDistanceSq(
  a: { r: number; g: number; b: number },
  b: { r: number; g: number; b: number },
): number {
  const dr = a.r - b.r;
  const dg = a.g - b.g;
  const db = a.b - b.b;
  return dr * dr + dg * dg + db * db;
}

export async function extractBrandColorsFromImage(imageUrl: string): Promise<ExtractedBrandColors | null> {
  if (!imageUrl) return null;

  const img = await new Promise<HTMLImageElement>((resolve, reject) => {
    const next = new Image();
    next.crossOrigin = "anonymous";
    next.onload = () => resolve(next);
    next.onerror = () => reject(new Error("Failed to load image"));
    next.src = imageUrl;
  });

  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  if (!ctx) return null;

  const targetWidth = 96;
  const scale = targetWidth / Math.max(1, img.width);
  const width = targetWidth;
  const height = Math.max(1, Math.round(img.height * scale));
  canvas.width = width;
  canvas.height = height;

  ctx.clearRect(0, 0, width, height);
  ctx.drawImage(img, 0, 0, width, height);
  const pixels = ctx.getImageData(0, 0, width, height).data;

  const buckets = new Map<string, ColorBucket>();

  for (let i = 0; i < pixels.length; i += 4 * 2) {
    const r = pixels[i];
    const g = pixels[i + 1];
    const b = pixels[i + 2];
    const a = pixels[i + 3];
    if (a < 128) continue;

    const { s, l } = rgbToHsl(r, g, b);
    if (s < 0.12) continue;
    if (l < 0.08 || l > 0.92) continue;

    const rq = Math.round(r / QUANT_STEP) * QUANT_STEP;
    const gq = Math.round(g / QUANT_STEP) * QUANT_STEP;
    const bq = Math.round(b / QUANT_STEP) * QUANT_STEP;
    const key = `${rq}-${gq}-${bq}`;
    const existing = buckets.get(key);
    if (existing) {
      existing.rSum += r;
      existing.gSum += g;
      existing.bSum += b;
      existing.count += 1;
    } else {
      buckets.set(key, { rSum: r, gSum: g, bSum: b, count: 1 });
    }
  }

  if (buckets.size === 0) return null;

  const ranked = Array.from(buckets.values())
    .map((bucket) => {
      const r = Math.round(bucket.rSum / bucket.count);
      const g = Math.round(bucket.gSum / bucket.count);
      const b = Math.round(bucket.bSum / bucket.count);
      const { s, l } = rgbToHsl(r, g, b);
      // Favor colorful, mid-light tones and recurring colors.
      const score = bucket.count * (0.6 + s) * (1 - Math.abs(l - 0.5));
      return { r, g, b, count: bucket.count, score };
    })
    .sort((a, b) => b.score - a.score);

  const primary = ranked[0];
  const secondary = ranked.find((entry) => colorDistanceSq(entry, primary) > 70 * 70) || ranked[1] || primary;

  return {
    primary: rgbToHex(primary.r, primary.g, primary.b),
    secondary: rgbToHex(secondary.r, secondary.g, secondary.b),
  };
}
