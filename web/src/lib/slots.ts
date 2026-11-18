import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import i18n from '../i18n';
import type { EventDetail, Mode, Slot, SlotInput } from './types';

dayjs.extend(utc);

const fmt = (key: 'fullDate' | 'shortDate' | 'dayMonth' | 'dayMonthYear' | 'cell') => i18n.t(`formats.${key}`);

/** Formats a date with a localized pattern from the translations. */
export function formatDate(d: dayjs.Dayjs, key: Parameters<typeof fmt>[0]): string {
  return d.format(fmt(key));
}

/** Day slots are stored as UTC midnights; this returns their calendar date. */
export function dayOf(slot: { start: string }, mode: Mode) {
  return mode === 'days' ? dayjs.utc(slot.start) : dayjs(slot.start);
}

export function formatSlotDay(slot: { start: string }, mode: Mode): { weekday: string; day: string; month: string } {
  const d = dayOf(slot, mode);
  return { weekday: d.format('ddd'), day: d.format('D'), month: d.format('MMM') };
}

export function formatSlotTime(slot: { start: string; end: string }, mode: Mode): string {
  if (mode === 'days') return i18n.t('vote.allDay');
  return `${dayjs(slot.start).format('HH:mm')} – ${dayjs(slot.end).format('HH:mm')}`;
}

export function formatSlot(slot: { start: string; end: string }, mode: Mode): string {
  const d = formatDate(dayOf(slot, mode), 'fullDate');
  return mode === 'days' ? d : `${d} · ${formatSlotTime(slot, mode)}`;
}

export function formatRange(first: string | undefined, last: string | undefined, mode: Mode): string {
  if (!first || !last) return i18n.t('dashboard.noDates');
  const a = mode === 'days' ? dayjs.utc(first) : dayjs(first);
  // Day slots end at the next midnight; show the last included day.
  const b = mode === 'days' ? dayjs.utc(last).subtract(1, 'day') : dayjs(last);
  if (a.isSame(b, 'day')) return formatDate(a, 'dayMonthYear');
  if (a.year() === b.year()) return `${formatDate(a, 'dayMonth')} – ${formatDate(b, 'dayMonthYear')}`;
  return `${formatDate(a, 'dayMonthYear')} – ${formatDate(b, 'dayMonthYear')}`;
}

/** Formats a duration in minutes, e.g. "2h 30" or "2 h 30". */
export function formatDuration(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (!h) return i18n.t('duration.minutes', { m });
  return m ? i18n.t('duration.hoursMinutes', { h, mm: String(m).padStart(2, '0') }) : i18n.t('duration.hours', { h });
}

/** Converts a local calendar date ("YYYY-MM-DD") to a whole-day slot. */
export function daySlot(date: string): SlotInput {
  const start = dayjs.utc(date);
  return { start: start.toISOString(), end: start.add(1, 'day').toISOString() };
}

export function slotDate(slot: { start: string }): string {
  return dayjs.utc(slot.start).format('YYYY-MM-DD');
}

export function slotKey(slot: { start: string; end: string }): string {
  return `${new Date(slot.start).getTime()}-${new Date(slot.end).getTime()}`;
}

export function sortSlots<T extends { start: string; end: string }>(slots: T[]): T[] {
  return [...slots].sort((a, b) => a.start.localeCompare(b.start) || a.end.localeCompare(b.end));
}

export interface SlotScore {
  slot: Slot;
  yes: number;
  maybe: number;
  score: number;
}

/** Scores every slot: a "yes" counts 1, a "maybe" counts 0.5. */
export function scoreSlots(event: Pick<EventDetail, 'slots' | 'responses'>): SlotScore[] {
  return event.slots.map((slot) => {
    let yes = 0;
    let maybe = 0;
    for (const r of event.responses) {
      if (r.votes[slot.id] === 'yes') yes++;
      else if (r.votes[slot.id] === 'maybe') maybe++;
    }
    return { slot, yes, maybe, score: yes + maybe / 2 };
  });
}

/** Returns the IDs of the slots with the highest (non-zero) score. */
export function bestSlotIds(scores: SlotScore[]): Set<string> {
  const max = Math.max(0, ...scores.map((s) => s.score));
  return new Set(max === 0 ? [] : scores.filter((s) => s.score === max).map((s) => s.slot.id));
}
