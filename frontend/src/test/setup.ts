import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});

if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// React Flow measures its container and nodes through browser APIs that jsdom
// does not implement. These shims provide stable dimensions so the graph can
// render in tests; they do not affect application code.
if (!globalThis.ResizeObserver) {
  class ResizeObserverMock implements ResizeObserver {
    constructor(private readonly callback: ResizeObserverCallback) {}

    observe(target: Element): void {
      const contentRect = {
        x: 0,
        y: 0,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        width: 800,
        height: 600,
        toJSON: () => ({}),
      } as DOMRectReadOnly;

      this.callback(
        [
          {
            target,
            contentRect,
            borderBoxSize: [],
            contentBoxSize: [],
            devicePixelContentBoxSize: [],
          } as unknown as ResizeObserverEntry,
        ],
        this,
      );
    }

    unobserve(): void {}

    disconnect(): void {}
  }

  globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
}

if (!globalThis.DOMMatrixReadOnly) {
  class DOMMatrixReadOnlyMock {
    m22 = 1;
  }

  globalThis.DOMMatrixReadOnly = DOMMatrixReadOnlyMock as unknown as typeof DOMMatrixReadOnly;
}

if (!("getBBox" in SVGElement.prototype)) {
  Object.defineProperty(SVGElement.prototype, "getBBox", {
    configurable: true,
    value: () => ({ x: 0, y: 0, width: 0, height: 0 }),
  });
}

Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
  configurable: true,
  get: () => 800,
});

Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
  configurable: true,
  get: () => 600,
});
