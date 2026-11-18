import {
  ActionIcon,
  Badge,
  Box,
  Button,
  CloseButton,
  Group,
  Paper,
  ScrollArea,
  Select,
  Stack,
  Text,
} from '@mantine/core';
import { DatePickerInput, TimeInput } from '@mantine/dates';
import { IconChevronLeft, IconChevronRight, IconPlus } from '@tabler/icons-react';
import dayjs, { type Dayjs } from 'dayjs';
import { useEffect, useEffectEvent, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatDate, formatDuration, slotKey, sortSlots } from '../lib/slots';
import type { SlotInput } from '../lib/types';
import classes from './HourSlotPicker.module.css';

interface Props {
  value: SlotInput[];
  onChange: (slots: SlotInput[]) => void;
  today?: Date;
}

interface Drag {
  day: number;
  from: number; // row index where the drag started
  to: number; // row index currently hovered
}

const hours = Array.from({ length: 25 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, '0')}:00` }));
const rowHeights: Record<string, number> = { '15': 16, '30': 22, '60': 36 };
const lengths = ['30', '60', '90', '120', '180', '240'];
const steps = ['15', '30', '60'];
const durationOptions = (values: string[]) => values.map((value) => ({ value, label: formatDuration(Number(value)) }));

function startOfWeek(d: Dayjs): Dayjs {
  // Weeks start on Monday.
  return d.startOf('day').subtract((d.day() + 6) % 7, 'day');
}

const hhmm = (d: Dayjs) => d.format('HH:mm');

/** The hour (rounded up) at which a slot ends on its start day. */
function endHour(slot: SlotInput): number {
  const end = dayjs(slot.end);
  if (!end.isSame(dayjs(slot.start), 'day')) return 24;
  return Math.ceil(end.diff(end.startOf('day'), 'minute') / 60);
}

/** Assigns overlapping slots of a day to side-by-side lanes. */
function layoutLanes(slots: SlotInput[]): { slot: SlotInput; lane: number; lanes: number }[] {
  const laneEnds: number[] = [];
  const placed = sortSlots(slots).map((slot) => {
    const start = Date.parse(slot.start);
    let lane = laneEnds.findIndex((end) => end <= start);
    if (lane === -1) lane = laneEnds.length;
    laneEnds[lane] = Date.parse(slot.end);
    return { slot, lane };
  });
  return placed.map((p) => ({ ...p, lanes: laneEnds.length }));
}

export function HourSlotPicker({ value, onChange, today = new Date() }: Props) {
  const { t } = useTranslation();
  const now = dayjs(today);
  const sorted = sortSlots(value);
  const [week, setWeek] = useState(() => startOfWeek(sorted.length ? dayjs(sorted[0].start) : now));
  const [step, setStep] = useState('30');
  const [length, setLength] = useState('60');
  // Day bounds cover existing slots so they are visible when editing.
  const [from, setFrom] = useState(() => String(Math.min(8, ...value.map((s) => dayjs(s.start).hour()))));
  const [to, setTo] = useState(() => String(Math.max(20, ...value.map(endHour))));
  const [drag, setDrag] = useState<Drag | null>(null);

  const [customDate, setCustomDate] = useState<string | null>(null);
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');
  const [customError, setCustomError] = useState<string | null>(null);

  const stepMin = Number(step);
  const rowHeight = rowHeights[step];
  const dayStartMin = Number(from) * 60;
  const dayEndMin = Number(to) * 60;
  const days = Array.from({ length: 7 }, (_, i) => week.add(i, 'day'));
  const rows = useMemo(() => {
    const out: number[] = [];
    for (let m = dayStartMin; m + stepMin <= dayEndMin; m += stepMin) out.push(m);
    return out;
  }, [stepMin, dayStartMin, dayEndMin]);

  const add = (slot: SlotInput): boolean => {
    if (value.some((s) => slotKey(s) === slotKey(slot))) return false;
    onChange(sortSlots([...value, slot]));
    return true;
  };
  const remove = (slot: SlotInput) => onChange(value.filter((s) => slotKey(s) !== slotKey(slot)));

  const cellStart = (day: number, row: number) => days[day].add(rows[row], 'minute');
  const dragRange = (d: Drag) => {
    const start = cellStart(d.day, Math.min(d.from, d.to));
    const end =
      d.from === d.to
        ? start.add(Number(length), 'minute')
        : cellStart(d.day, Math.max(d.from, d.to)).add(stepMin, 'minute');
    return { start, end };
  };

  // A drag ends wherever the mouse is released, even outside the grid.
  const finishDrag = useEffectEvent(() => {
    if (!drag) return;
    setDrag(null);
    const { start, end } = dragRange(drag);
    add({ start: start.toISOString(), end: end.toISOString() });
  });
  useEffect(() => {
    const stop = () => finishDrag();
    window.addEventListener('mouseup', stop);
    return () => window.removeEventListener('mouseup', stop);
  }, []);

  const block = (start: Dayjs, end: Dayjs, day: Dayjs) => {
    const startMin = start.diff(day, 'minute');
    const endMin = end.diff(day, 'minute');
    const top = Math.max(startMin, dayStartMin);
    const bottom = Math.min(endMin, dayEndMin);
    if (bottom <= top) return null;
    return { top: ((top - dayStartMin) / stepMin) * rowHeight, height: ((bottom - top) / stepMin) * rowHeight };
  };

  const addCustom = () => {
    const start = dayjs(`${customDate}T${customStart}`);
    const end = dayjs(`${customDate}T${customEnd}`);
    if (!customDate || !start.isValid() || !end.isValid()) {
      setCustomError(t('hours.errorIncomplete'));
      return;
    }
    if (!end.isAfter(start)) {
      setCustomError(t('hours.errorOrder'));
      return;
    }
    if (!add({ start: start.toISOString(), end: end.toISOString() })) {
      setCustomError(t('hours.errorDuplicate'));
      return;
    }
    setCustomError(null);
    setWeek(startOfWeek(start));
    // Make sure the new slot is visible in the grid.
    setFrom(String(Math.min(Number(from), start.hour())));
    setTo(String(Math.max(Number(to), endHour({ start: start.toISOString(), end: end.toISOString() }))));
  };

  const grouped = new Map<string, SlotInput[]>();
  for (const s of sorted) {
    const day = formatDate(dayjs(s.start), 'shortDate');
    grouped.set(day, [...(grouped.get(day) ?? []), s]);
  }
  const preview = drag && dragRange(drag);

  return (
    <Stack gap="md">
      <Group justify="space-between" gap="sm">
        <Group gap={6}>
          <ActionIcon
            variant="default"
            size="lg"
            aria-label={t('hours.previousWeek')}
            onClick={() => setWeek(week.subtract(7, 'day'))}
          >
            <IconChevronLeft size={18} />
          </ActionIcon>
          <Button variant="default" onClick={() => setWeek(startOfWeek(now))}>
            {t('hours.today')}
          </Button>
          <ActionIcon
            variant="default"
            size="lg"
            aria-label={t('hours.nextWeek')}
            onClick={() => setWeek(week.add(7, 'day'))}
          >
            <IconChevronRight size={18} />
          </ActionIcon>
          <Text fw={600} ml="xs" data-testid="week-label">
            {formatDate(week, 'dayMonth')} – {formatDate(week.add(6, 'day'), 'dayMonthYear')}
          </Text>
        </Group>
        <Group gap="xs" align="flex-end">
          <Select
            label={t('hours.clickAdds')}
            aria-label={t('hours.defaultLength')}
            size="xs"
            w={104}
            value={length}
            onChange={(v) => setLength(v ?? '60')}
            allowDeselect={false}
            data={durationOptions(lengths)}
          />
          <Select
            label={t('hours.grid')}
            aria-label={t('hours.gridStep')}
            size="xs"
            w={92}
            value={step}
            onChange={(v) => setStep(v ?? '30')}
            allowDeselect={false}
            data={durationOptions(steps)}
          />
          <Select
            label={t('hours.from')}
            aria-label={t('hours.dayStartsAt')}
            size="xs"
            w={84}
            value={from}
            onChange={(v) => setFrom(v ?? '8')}
            allowDeselect={false}
            data={hours.slice(0, 24).filter((h) => Number(h.value) < Number(to))}
          />
          <Select
            label={t('hours.to')}
            aria-label={t('hours.dayEndsAt')}
            size="xs"
            w={84}
            value={to}
            onChange={(v) => setTo(v ?? '20')}
            allowDeselect={false}
            data={hours.slice(1).filter((h) => Number(h.value) > Number(from))}
          />
        </Group>
      </Group>

      <Paper withBorder p={0} style={{ overflow: 'hidden' }}>
        <ScrollArea>
          <div className={classes.wrap} data-dragging={drag ? true : undefined}>
            <div className={classes.header}>
              <div />
              {days.map((d) => (
                <div key={d.toISOString()} className={classes.dayHeader} data-today={d.isSame(now, 'day') || undefined}>
                  <Text fz="xs" c="dimmed" tt="uppercase" fw={600}>
                    {d.format('ddd')}
                  </Text>
                  <Text fz="lg" fw={700} lh={1.2}>
                    {d.format('D')}
                  </Text>
                </div>
              ))}
            </div>
            <div className={classes.body} role="grid" aria-label={t('hours.weekCalendar')}>
              <div className={classes.times}>
                {rows.map((m) => (
                  <div key={m} className={classes.time} style={{ height: rowHeight }}>
                    {m % 60 === 0 && `${String(m / 60).padStart(2, '0')}:00`}
                  </div>
                ))}
              </div>
              {days.map((d, di) => (
                <div key={d.toISOString()} className={classes.column} role="row">
                  {rows.map((m, ri) => {
                    const start = d.add(m, 'minute');
                    return (
                      <button
                        type="button"
                        key={m}
                        role="gridcell"
                        aria-label={formatDate(start, 'cell')}
                        disabled={start.isBefore(now)}
                        className={classes.cell}
                        data-hour={m % 60 === 0 || undefined}
                        style={{ height: rowHeight }}
                        onMouseDown={(e) => {
                          e.preventDefault();
                          setDrag({ day: di, from: ri, to: ri });
                        }}
                        onMouseEnter={() => {
                          setDrag((cur) => (cur && cur.day === di ? { ...cur, to: ri } : cur));
                        }}
                        onClick={(e) => {
                          // Keyboard activation (mouse is handled by mousedown/mouseup).
                          if (e.detail === 0) {
                            add({ start: start.toISOString(), end: start.add(Number(length), 'minute').toISOString() });
                          }
                        }}
                      />
                    );
                  })}
                  {layoutLanes(value.filter((s) => dayjs(s.start).isSame(d, 'day'))).map(({ slot, lane, lanes }) => {
                    const pos = block(dayjs(slot.start), dayjs(slot.end), d);
                    if (!pos) return null;
                    const label = `${hhmm(dayjs(slot.start))}–${hhmm(dayjs(slot.end))}`;
                    return (
                      <div
                        key={slotKey(slot)}
                        className={classes.slot}
                        data-testid="slot-block"
                        style={{
                          ...pos,
                          left: `calc(${(lane / lanes) * 100}% + 2px)`,
                          width: `calc(${100 / lanes}% - 4px)`,
                        }}
                        onMouseDown={(e) => e.stopPropagation()}
                        title={`${label} (${formatDuration(dayjs(slot.end).diff(dayjs(slot.start), 'minute'))})`}
                      >
                        <span className={classes.slotLabel}>{label}</span>
                        <CloseButton
                          size="xs"
                          variant="transparent"
                          className={classes.slotRemove}
                          aria-label={t('hours.removeSlot', { label: `${formatDate(d, 'shortDate')} ${label}` })}
                          onClick={() => remove(slot)}
                        />
                      </div>
                    );
                  })}
                  {preview && drag.day === di && (
                    <div
                      className={classes.preview}
                      style={block(preview.start, preview.end, d) ?? undefined}
                      data-testid="drag-preview"
                    >
                      {hhmm(preview.start)}–{hhmm(preview.end)}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </ScrollArea>
      </Paper>
      <Text fz="xs" c="dimmed">
        {t('hours.tip', {
          length: formatDuration(Number(length)),
          tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
        })}
      </Text>

      <Paper withBorder p="sm">
        <Text fz="sm" fw={600} mb={6}>
          {t('hours.exactTitle')}
        </Text>
        <Group gap="xs" align="flex-start">
          <DatePickerInput
            aria-label={t('hours.slotDate')}
            placeholder={t('hours.datePlaceholder')}
            size="xs"
            w={150}
            value={customDate}
            onChange={setCustomDate}
            minDate={now.format('YYYY-MM-DD')}
          />
          <TimeInput
            aria-label={t('hours.slotStart')}
            size="xs"
            w={100}
            value={customStart}
            onChange={(e) => setCustomStart(e.currentTarget.value)}
          />
          <TimeInput
            aria-label={t('hours.slotEnd')}
            size="xs"
            w={100}
            value={customEnd}
            onChange={(e) => setCustomEnd(e.currentTarget.value)}
          />
          <Button size="xs" variant="light" leftSection={<IconPlus size={14} />} onClick={addCustom}>
            {t('hours.addSlot')}
          </Button>
        </Group>
        {customError && (
          <Text c="red" fz="xs" mt={6} role="alert">
            {customError}
          </Text>
        )}
      </Paper>

      <div>
        <Text fz="sm" fw={600} mb={6}>
          {t('hours.selected', { count: value.length })}
        </Text>
        <Stack gap={6}>
          {[...grouped.entries()].map(([day, slots]) => (
            <Group key={day} gap={6}>
              <Text fz="sm" w={110} c="dimmed">
                {day}
              </Text>
              {slots.map((s) => {
                const label = `${hhmm(dayjs(s.start))}–${hhmm(dayjs(s.end))}`;
                return (
                  <Badge
                    key={slotKey(s)}
                    variant="light"
                    radius="sm"
                    size="lg"
                    tt="none"
                    rightSection={
                      <CloseButton
                        size="xs"
                        variant="transparent"
                        aria-label={t('hours.remove', { label: `${day} ${label}` })}
                        onClick={() => remove(s)}
                      />
                    }
                  >
                    {label}
                    <Box component="span" c="dimmed" fw={400} ml={6}>
                      {formatDuration(dayjs(s.end).diff(dayjs(s.start), 'minute'))}
                    </Box>
                  </Badge>
                );
              })}
            </Group>
          ))}
        </Stack>
      </div>
    </Stack>
  );
}
