import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { notifications } from '@mantine/notifications';
import { afterEach, beforeEach } from 'vitest';
import i18n from '../i18n';

beforeEach(async () => {
  await i18n.changeLanguage('en');
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  notifications.clean();
  notifications.cleanQueue();
});

const { getComputedStyle } = window;
window.getComputedStyle = (elt) => getComputedStyle(elt);
window.HTMLElement.prototype.scrollIntoView = () => {};

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});

class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
window.ResizeObserver = ResizeObserver;

Object.defineProperty(document, 'fonts', {
  value: { addEventListener: () => {}, removeEventListener: () => {}, ready: Promise.resolve() },
});

// jsdom has no object URLs; previews only need a stable string.
URL.createObjectURL = (obj: Blob | MediaSource) => `blob:${obj instanceof File ? obj.name : 'preview'}`;
URL.revokeObjectURL = () => {};

// jsdom does not implement pointer capture.
HTMLElement.prototype.setPointerCapture = () => {};
