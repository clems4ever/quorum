import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useApi } from './api';

describe('useApi', () => {
  it('requires a provider', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    function Probe() {
      useApi();
      return null;
    }
    expect(() => render(<Probe />)).toThrow('useApi must be used inside');
  });
});
