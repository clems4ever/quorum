import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { SlotInput } from '../lib/types';
import { DaySlotPicker } from './DaySlotPicker';

function Harness({ initial = [], spy }: { initial?: SlotInput[]; spy?: (s: SlotInput[]) => void }) {
  const [value, setValue] = useState<SlotInput[]>(initial);
  return (
    <MantineProvider>
      <DaySlotPicker
        value={value}
        today={new Date('2026-11-18T12:00:00Z')}
        onChange={(v) => {
          spy?.(v);
          setValue(v);
        }}
      />
    </MantineProvider>
  );
}

const day = (label: string) => screen.getByRole('button', { name: label });

describe('DaySlotPicker', () => {
  it('selects and removes days while keeping existing slots', async () => {
    const spy = vi.fn();
    const existing = { id: 'keep', start: '2026-11-20T00:00:00Z', end: '2026-11-21T00:00:00Z' };
    render(<Harness initial={[existing]} spy={spy} />);
    expect(screen.getByText('1 day selected')).toBeInTheDocument();
    expect(day('17 November 2026')).toBeDisabled();

    await userEvent.click(day('19 November 2026'));
    expect(spy).toHaveBeenLastCalledWith([
      { start: '2026-11-19T00:00:00.000Z', end: '2026-11-20T00:00:00.000Z' },
      existing,
    ]);
    expect(screen.getByText('2 days selected')).toBeInTheDocument();
    expect(screen.getByText('Thu, Nov 19')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Remove 2026-11-19' }));
    expect(spy).toHaveBeenLastCalledWith([existing]);

    await userEvent.click(day('20 November 2026'));
    expect(spy).toHaveBeenLastCalledWith([]);
  });
});
