import { describe, expect, it } from 'vitest';
import { CENTER, checkImage, dragFocus, MAX_IMAGE_BYTES, nudgeFocus, objectPosition } from './cover';

const file = (type: string, size = 10) => {
  const f = new File(['x'], 'f', { type });
  Object.defineProperty(f, 'size', { value: size });
  return f;
};

describe('cover helpers', () => {
  it('checks files', () => {
    expect(checkImage(file('image/png'))).toBeNull();
    expect(checkImage(file('image/svg+xml'))).toBe('invalidType');
    expect(checkImage(file('image/jpeg', MAX_IMAGE_BYTES + 1))).toBe('tooLarge');
  });

  it('formats object positions', () => {
    expect(objectPosition()).toBe('50% 50%');
    expect(objectPosition({ x: 0.1234, y: 1 })).toBe('12.34% 100%');
  });

  it('drags the image like a physical photo', () => {
    // A 1600x900 image covering a 1000x200 box is shown at 1000x562.5:
    // it overflows vertically by 362.5px and not at all horizontally.
    const box = { width: 1000, height: 200 };
    const image = { width: 1600, height: 900 };
    expect(dragFocus(CENTER, 0, -145, box, image)).toEqual({ x: 0.5, y: 0.9 });
    expect(dragFocus(CENTER, 300, 145, box, image)).toEqual({ x: 0.5, y: 0.1 });
    // Clamped at the edges.
    expect(dragFocus(CENTER, 0, -10_000, box, image)).toEqual({ x: 0.5, y: 1 });
    expect(dragFocus(CENTER, 0, 10_000, box, image)).toEqual({ x: 0.5, y: 0 });
    // A tall image overflows horizontally in a tall box.
    expect(dragFocus(CENTER, 50, 0, { width: 100, height: 400 }, { width: 400, height: 400 })).toEqual({
      x: 0.333,
      y: 0.5,
    });
  });

  it('nudges with the arrow keys', () => {
    expect(nudgeFocus(CENTER, 'ArrowLeft', 0.1)).toEqual({ x: 0.4, y: 0.5 });
    expect(nudgeFocus(CENTER, 'ArrowRight', 0.1)).toEqual({ x: 0.6, y: 0.5 });
    expect(nudgeFocus(CENTER, 'ArrowUp', 0.1)).toEqual({ x: 0.5, y: 0.4 });
    expect(nudgeFocus({ x: 0.5, y: 0.95 }, 'ArrowDown', 0.1)).toEqual({ x: 0.5, y: 1 });
    expect(nudgeFocus(CENTER, 'Enter', 0.1)).toBeNull();
  });
});
