import { describe, expect, it } from 'vitest';
import {
  bestSlotIds,
  daySlot,
  formatRange,
  formatSlot,
  formatSlotDay,
  formatDuration,
  formatSlotTime,
  scoreSlots,
  slotDate,
  slotKey,
  sortSlots,
} from './slots';

const hourSlot = { start: '2026-12-01T09:00:00Z', end: '2026-12-01T10:30:00Z' };
const day = { start: '2026-12-01T00:00:00Z', end: '2026-12-02T00:00:00Z' };

describe('slots', () => {
  it('formats slots', () => {
    expect(formatSlotDay(day, 'days')).toEqual({ weekday: 'Tue', day: '1', month: 'Dec' });
    expect(formatSlotTime(day, 'days')).toBe('All day');
    expect(formatSlotTime(hourSlot, 'hours')).toBe('09:00 – 10:30');
    expect(formatSlot(day, 'days')).toBe('Tue, Dec 1, 2026');
    expect(formatSlot(hourSlot, 'hours')).toBe('Tue, Dec 1, 2026 · 09:00 – 10:30');
  });

  it('formats ranges', () => {
    expect(formatRange(undefined, undefined, 'days')).toBe('No dates');
    expect(formatRange(day.start, day.end, 'days')).toBe('Dec 1, 2026');
    expect(formatRange(day.start, '2026-12-05T00:00:00Z', 'days')).toBe('Dec 1 – Dec 4, 2026');
    expect(formatRange(hourSlot.start, '2027-01-02T10:00:00Z', 'hours')).toBe('Dec 1, 2026 – Jan 2, 2027');
  });

  it('converts dates to day slots and back', () => {
    expect(daySlot('2026-12-01')).toEqual({ start: '2026-12-01T00:00:00.000Z', end: '2026-12-02T00:00:00.000Z' });
    expect(slotDate(day)).toBe('2026-12-01');
  });

  it('keys and sorts slots', () => {
    expect(slotKey(day)).toBe(`${Date.parse(day.start)}-${Date.parse(day.end)}`);
    expect(sortSlots([hourSlot, day])).toEqual([day, hourSlot]);
    const sameStart = { start: day.start, end: '2026-12-01T05:00:00Z' };
    expect(sortSlots([day, sameStart])).toEqual([sameStart, day]);
  });

  it('scores slots and finds the best ones', () => {
    const event = {
      slots: [
        { id: 'a', ...day },
        { id: 'b', ...day },
        { id: 'c', ...day },
      ],
      responses: [
        { user: { id: '1', name: '', email: '', picture: '' }, votes: { a: 'yes', b: 'maybe' }, updatedAt: '' },
        { user: { id: '2', name: '', email: '', picture: '' }, votes: { a: 'maybe', b: 'yes' }, updatedAt: '' },
      ],
    } as const;
    const scores = scoreSlots(event as never);
    expect(scores.map((s) => [s.slot.id, s.yes, s.maybe, s.score])).toEqual([
      ['a', 1, 1, 1.5],
      ['b', 1, 1, 1.5],
      ['c', 0, 0, 0],
    ]);
    expect([...bestSlotIds(scores)]).toEqual(['a', 'b']);
    expect(bestSlotIds(scores.slice(2)).size).toBe(0);
  });
});

describe('formatDuration', () => {
  it('formats minutes', () => {
    expect(formatDuration(30)).toBe('30 min');
    expect(formatDuration(60)).toBe('1h');
    expect(formatDuration(150)).toBe('2h 30');
  });
});
