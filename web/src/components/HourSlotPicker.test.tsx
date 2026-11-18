import { MantineProvider } from '@mantine/core';
import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { SlotInput } from '../lib/types';
import { HourSlotPicker } from './HourSlotPicker';

const today = new Date('2026-11-18T12:10:00Z'); // a Wednesday

function Harness({ initial = [], spy }: { initial?: SlotInput[]; spy?: (s: SlotInput[]) => void }) {
  const [value, setValue] = useState<SlotInput[]>(initial);
  return (
    <MantineProvider>
      <HourSlotPicker
        value={value}
        today={today}
        onChange={(v) => {
          spy?.(v);
          setValue(v);
        }}
      />
    </MantineProvider>
  );
}

const cell = (label: string) => screen.getByRole('gridcell', { name: label });
const slot = (start: string, end: string) => ({ start: `2026-${start}:00.000Z`, end: `2026-${end}:00.000Z` });

function drag(from: string, to: string) {
  fireEvent.mouseDown(cell(from));
  fireEvent.mouseEnter(cell(to));
  fireEvent.mouseUp(window);
}

async function choose(label: string, option: string) {
  const input = screen.getAllByLabelText(label)[0];
  await userEvent.click(input);
  const listbox = document.getElementById(input.getAttribute('aria-controls')!)!;
  await userEvent.click(within(listbox).getByRole('option', { name: option, hidden: true }));
}

describe('HourSlotPicker', () => {
  it('shows the current week starting on Monday with past cells disabled', () => {
    render(<Harness />);
    expect(screen.getByTestId('week-label')).toHaveTextContent('Nov 16 – Nov 22, 2026');
    expect(cell('Wed Nov 18 12:00')).toBeDisabled();
    expect(cell('Wed Nov 18 12:30')).toBeEnabled();
    expect(within(screen.getAllByRole('row')[0]).getAllByRole('gridcell')).toHaveLength(24); // 08:00..19:30
    expect(screen.getByText('0 slots selected')).toBeInTheDocument();
  });

  it('adds a slot of the default length on click', () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    drag('Thu Nov 19 09:00', 'Thu Nov 19 09:00');
    expect(spy).toHaveBeenLastCalledWith([slot('11-19T09:00', '11-19T10:00')]);
    expect(screen.getByText('1 slot selected')).toBeInTheDocument();
    expect(screen.getAllByTestId('slot-block')).toHaveLength(1);
    // Adding the same slot twice is ignored.
    drag('Thu Nov 19 09:00', 'Thu Nov 19 09:00');
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('creates slots of any length by dragging, in both directions', () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    fireEvent.mouseDown(cell('Thu Nov 19 10:00'));
    fireEvent.mouseEnter(cell('Thu Nov 19 12:00'));
    fireEvent.mouseEnter(cell('Fri Nov 20 15:00')); // other columns are ignored
    expect(screen.getByTestId('drag-preview')).toHaveTextContent('10:00–12:30');
    fireEvent.mouseUp(window);
    expect(screen.queryByTestId('drag-preview')).not.toBeInTheDocument();
    expect(spy).toHaveBeenLastCalledWith([slot('11-19T10:00', '11-19T12:30')]);

    drag('Thu Nov 19 15:00', 'Thu Nov 19 13:30');
    expect(spy).toHaveBeenLastCalledWith([slot('11-19T10:00', '11-19T12:30'), slot('11-19T13:30', '11-19T15:30')]);
    expect(screen.getByText('2h 30')).toBeInTheDocument();
    expect(screen.getByText('2 slots selected')).toBeInTheDocument();

    // A mouseup without a drag does nothing.
    fireEvent.mouseUp(window);
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('lays out overlapping slots side by side and removes them', async () => {
    render(
      <Harness
        initial={[
          slot('11-19T10:00', '11-19T12:00'),
          slot('11-19T11:00', '11-19T13:00'),
          slot('11-19T13:00', '11-19T14:00'),
        ]}
      />,
    );
    const blocks = screen.getAllByTestId('slot-block');
    expect(blocks.map((b) => b.style.width)).toEqual(['calc(50% - 4px)', 'calc(50% - 4px)', 'calc(50% - 4px)']);
    expect(blocks.map((b) => b.style.left)).toEqual(['calc(0% + 2px)', 'calc(50% + 2px)', 'calc(0% + 2px)']);
    // Blocks do not start a new selection.
    fireEvent.mouseDown(blocks[0]);
    await userEvent.click(screen.getByRole('button', { name: 'Remove slot Thu, Nov 19 11:00–13:00' }));
    expect(screen.getByText('2 slots selected')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Remove Thu, Nov 19 13:00–14:00' }));
    expect(screen.getByText('1 slot selected')).toBeInTheDocument();
  });

  it('supports keyboard activation and ignores mouse clicks', () => {
    render(<Harness />);
    fireEvent.click(cell('Fri Nov 20 08:00'), { detail: 1 });
    expect(screen.getByText('0 slots selected')).toBeInTheDocument();
    fireEvent.click(cell('Fri Nov 20 08:00'), { detail: 0 });
    expect(screen.getByText('1 slot selected')).toBeInTheDocument();
  });

  it('navigates weeks and fits existing slots in the grid', async () => {
    render(
      <Harness
        initial={[
          { id: 'x', start: '2026-12-02T06:00:00Z', end: '2026-12-02T22:30:00Z' },
          slot('12-03T23:00', '12-04T01:00'),
        ]}
      />,
    );
    expect(screen.getByTestId('week-label')).toHaveTextContent('Nov 30 – Dec 6, 2026');
    expect(screen.getAllByLabelText('Day starts at')[0]).toHaveValue('06:00');
    expect(screen.getAllByLabelText('Day ends at')[0]).toHaveValue('24:00');
    await userEvent.click(screen.getByRole('button', { name: 'Next week' }));
    expect(screen.getByTestId('week-label')).toHaveTextContent('Dec 7 – Dec 13, 2026');
    expect(screen.queryAllByTestId('slot-block')).toHaveLength(0);
    await userEvent.click(screen.getByRole('button', { name: 'Previous week' }));
    await userEvent.click(screen.getByRole('button', { name: 'Previous week' }));
    expect(screen.getByTestId('week-label')).toHaveTextContent('Nov 23 – Nov 29, 2026');
    await userEvent.click(screen.getByRole('button', { name: 'Today' }));
    expect(screen.getByTestId('week-label')).toHaveTextContent('Nov 16 – Nov 22, 2026');
  });

  it('hides slots outside of the visible hours', () => {
    render(<Harness initial={[slot('11-19T09:00', '11-19T10:00')]} />);
    expect(screen.getAllByTestId('slot-block')).toHaveLength(1);
  });

  it('changes grid step, default length and day bounds', async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await choose('Grid step', '1h');
    expect(within(screen.getAllByRole('row')[0]).getAllByRole('gridcell')).toHaveLength(12);
    await choose('Grid step', '15 min');
    expect(within(screen.getAllByRole('row')[0]).getAllByRole('gridcell')).toHaveLength(48);
    await choose('Default slot length', '2h');
    drag('Thu Nov 19 10:15', 'Thu Nov 19 10:15');
    expect(spy).toHaveBeenLastCalledWith([slot('11-19T10:15', '11-19T12:15')]);
    await choose('Day starts at', '06:00');
    await choose('Day ends at', '22:00');
    expect(within(screen.getAllByRole('row')[0]).getAllByRole('gridcell')).toHaveLength(64);
  });

  it('adds exact slots', async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    const add = screen.getByRole('button', { name: 'Add slot' });
    await userEvent.click(add);
    expect(screen.getByRole('alert')).toHaveTextContent('Pick a date');

    await userEvent.click(screen.getAllByLabelText('Slot date')[0]);
    await userEvent.click(await screen.findByRole('button', { name: '27 November 2026', hidden: true }));
    fireEvent.change(screen.getAllByLabelText('Slot start')[0], { target: { value: '19:00' } });
    fireEvent.change(screen.getAllByLabelText('Slot end')[0], { target: { value: '18:00' } });
    await userEvent.click(add);
    expect(screen.getByRole('alert')).toHaveTextContent('The end must be after the start');

    fireEvent.change(screen.getAllByLabelText('Slot end')[0], { target: { value: '23:30' } });
    await userEvent.click(add);
    expect(spy).toHaveBeenLastCalledWith([slot('11-27T19:00', '11-27T23:30')]);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getAllByLabelText('Day ends at')[0]).toHaveValue('24:00');
    expect(screen.getByTestId('week-label')).toHaveTextContent('Nov 23 – Nov 29, 2026');

    await userEvent.click(add);
    expect(screen.getByRole('alert')).toHaveTextContent('already selected');
  });
});
