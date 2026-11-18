import { Badge, CloseButton, Group, Paper, Stack, Text } from '@mantine/core';
import { DatePicker } from '@mantine/dates';
import { useMediaQuery } from '@mantine/hooks';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { daySlot, formatDate, slotDate, sortSlots } from '../lib/slots';
import type { SlotInput } from '../lib/types';

interface Props {
  value: SlotInput[];
  onChange: (slots: SlotInput[]) => void;
  today?: Date;
}

export function DaySlotPicker({ value, onChange, today = new Date() }: Props) {
  const { t } = useTranslation();
  const wide = useMediaQuery('(min-width: 62em)');
  const dates = value.map(slotDate);
  const byDate = new Map(value.map((s) => [slotDate(s), s]));

  const setDates = (next: string[]) => {
    // Preserve existing slots (and their IDs/votes) for dates still selected.
    onChange(sortSlots(next.map((d) => byDate.get(d) ?? daySlot(d))));
  };

  return (
    <Stack gap="md">
      <Paper withBorder p="md" style={{ display: 'flex', justifyContent: 'center' }}>
        <DatePicker
          type="multiple"
          value={dates}
          onChange={setDates}
          minDate={dayjs(today).format('YYYY-MM-DD')}
          numberOfColumns={wide ? 2 : 1}
          size="md"
          getDayProps={() => ({ 'aria-pressed': undefined })}
        />
      </Paper>
      <div>
        <Text fz="sm" fw={600} mb={6}>
          {t('days.selected', { count: value.length })}
        </Text>
        <Group gap={6}>
          {sortSlots(value).map((s) => (
            <Badge
              key={slotDate(s)}
              variant="light"
              size="lg"
              radius="sm"
              rightSection={
                <CloseButton
                  size="xs"
                  variant="transparent"
                  aria-label={t('days.remove', { date: slotDate(s) })}
                  onClick={() => setDates(dates.filter((d) => d !== slotDate(s)))}
                />
              }
            >
              {formatDate(dayjs.utc(s.start), 'shortDate')}
            </Badge>
          ))}
        </Group>
      </div>
    </Stack>
  );
}
