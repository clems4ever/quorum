import type { Focus } from './types';

/** What the editor will do with an event's cover image when saving. */
export type CoverChange = { kind: 'keep' } | { kind: 'set'; file: File } | { kind: 'remove' };

/** Same limit as the server. */
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024;

export const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];

export const CENTER: Focus = { x: 0.5, y: 0.5 };

/** Checks a file before uploading it; returns the problem, if any. */
export function checkImage(file: File): 'tooLarge' | 'invalidType' | null {
  if (!IMAGE_TYPES.includes(file.type)) return 'invalidType';
  if (file.size > MAX_IMAGE_BYTES) return 'tooLarge';
  return null;
}

/** CSS object-position keeping the focus visible in a cropped image. */
export function objectPosition(focus: Focus = CENTER): string {
  return `${round(focus.x * 100)}% ${round(focus.y * 100)}%`;
}

interface Size {
  width: number;
  height: number;
}

const clamp = (v: number) => Math.min(1, Math.max(0, v));
const round = (v: number) => Math.round(v * 1000) / 1000;

/**
 * Returns the focus after dragging an image displayed with object-fit: cover
 * by (dx, dy) pixels: the image follows the pointer, so dragging right
 * reveals more of its left side.
 */
export function dragFocus(start: Focus, dx: number, dy: number, box: Size, image: Size): Focus {
  const scale = Math.max(box.width / image.width, box.height / image.height);
  const overflowX = image.width * scale - box.width;
  const overflowY = image.height * scale - box.height;
  return {
    x: overflowX > 0 ? round(clamp(start.x - dx / overflowX)) : start.x,
    y: overflowY > 0 ? round(clamp(start.y - dy / overflowY)) : start.y,
  };
}

/** Moves the focus with the keyboard by a fraction of the image. */
export function nudgeFocus(focus: Focus, key: string, step: number): Focus | null {
  const moves: Record<string, [number, number]> = {
    ArrowLeft: [-step, 0],
    ArrowRight: [step, 0],
    ArrowUp: [0, -step],
    ArrowDown: [0, step],
  };
  const move = moves[key];
  if (!move) return null;
  return { x: round(clamp(focus.x + move[0])), y: round(clamp(focus.y + move[1])) };
}
