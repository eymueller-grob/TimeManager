import {cleanup} from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import {afterEach, vi} from 'vitest';

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })),
});

if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = class implements ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

afterEach(cleanup);

if (!HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function () {
    this.setAttribute('open', '');
  };
}

if (!HTMLDialogElement.prototype.close) {
  HTMLDialogElement.prototype.close = function () {
    this.removeAttribute('open');
  };
}
