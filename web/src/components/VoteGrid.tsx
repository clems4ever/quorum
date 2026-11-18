import { Avatar, Badge, Box, Group, ScrollArea, Table, Text, Tooltip, UnstyledButton } from '@mantine/core';
import { IconCheck, IconStarFilled, IconX } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { bestSlotIds, formatSlotDay, formatSlotTime, scoreSlots } from '../lib/slots';
import type { EventDetail, User, VoteOrNo } from '../lib/types';
import classes from './VoteGrid.module.css';

interface Props {
  event: EventDetail;
  me: User;
  /** The current user's (possibly unsaved) votes; undefined when not editable. */
  draft?: Record<string, VoteOrNo>;
  onCycle?: (slotId: string) => void;
}

export function VoteMark({ vote }: { vote: VoteOrNo }) {
  const { t } = useTranslation();
  const label = t(`vote.${vote}`);
  if (vote === 'yes') return <IconCheck size={18} stroke={3} aria-label={label} />;
  if (vote === 'maybe')
    return (
      <Text fz="sm" fw={800} aria-label={label}>
        (✓)
      </Text>
    );
  return <IconX size={14} stroke={2} aria-label={label} opacity={0.35} />;
}

function Person({ user, you }: { user: User; you?: boolean }) {
  const { t } = useTranslation();
  return (
    <Group gap={8} wrap="nowrap">
      <Avatar src={user.picture || null} name={user.name} color="initials" size={28} radius="xl" />
      <Text fz="sm" fw={you ? 700 : 500} truncate maw={160}>
        {you ? t('vote.you', { name: user.name }) : user.name}
      </Text>
    </Group>
  );
}

export function VoteGrid({ event, me, draft, onCycle }: Props) {
  const { t } = useTranslation();
  const scores = scoreSlots(event);
  const best = bestSlotIds(scores);
  const others = event.responses.filter((r) => r.user.id !== me.id);
  const mine = event.responses.find((r) => r.user.id === me.id);
  const colState = (id: string) =>
    event.finalSlotId === id ? 'final' : best.has(id) && !event.finalSlotId ? 'best' : undefined;

  return (
    <ScrollArea type="auto" offsetScrollbars>
      <Table className={classes.table} withRowBorders={false} verticalSpacing={6} horizontalSpacing={4}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th className={classes.sticky}>
              <Text fz="sm" c="dimmed" fw={500}>
                {t('vote.participants', { count: event.responses.length })}
              </Text>
            </Table.Th>
            {event.slots.map((s) => {
              const d = formatSlotDay(s, event.mode);
              const state = colState(s.id);
              return (
                <Table.Th key={s.id} className={classes.slotHead} data-state={state}>
                  <Text fz={11} tt="uppercase" fw={700} c="dimmed">
                    {d.month}
                  </Text>
                  <Text fz={24} fw={800} lh={1.1}>
                    {d.day}
                  </Text>
                  <Text fz="xs" fw={600}>
                    {d.weekday}
                  </Text>
                  {event.mode === 'hours' && (
                    <Text fz={11} c="dimmed" mt={2} style={{ whiteSpace: 'nowrap' }}>
                      {formatSlotTime(s, event.mode)}
                    </Text>
                  )}
                  {state === 'best' && (
                    <Tooltip label={t('vote.mostPopular')}>
                      <Box c="yellow.6" mt={4} data-testid={`best-${s.id}`}>
                        <IconStarFilled size={14} />
                      </Box>
                    </Tooltip>
                  )}
                  {state === 'final' && (
                    <Badge size="xs" color="teal" mt={4}>
                      {t('vote.chosen')}
                    </Badge>
                  )}
                </Table.Th>
              );
            })}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {draft && (
            <Table.Tr className={classes.mine}>
              <Table.Td className={classes.sticky}>
                <Person user={me} you />
              </Table.Td>
              {event.slots.map((s) => {
                const v = draft[s.id] ?? 'no';
                return (
                  <Table.Td key={s.id} className={classes.cellWrap} data-state={colState(s.id)}>
                    <UnstyledButton
                      className={classes.cell}
                      data-vote={v}
                      data-editable
                      aria-label={t('vote.yourVote', {
                        slot: `${formatSlotDay(s, event.mode).weekday} ${formatSlotDay(s, event.mode).day} ${formatSlotTime(s, event.mode)}`,
                        vote: t(`vote.${v}`),
                      })}
                      onClick={() => onCycle?.(s.id)}
                    >
                      <VoteMark vote={v} />
                    </UnstyledButton>
                  </Table.Td>
                );
              })}
            </Table.Tr>
          )}
          {!draft && mine && (
            <Table.Tr>
              <Table.Td className={classes.sticky}>
                <Person user={me} you />
              </Table.Td>
              {event.slots.map((s) => (
                <Table.Td key={s.id} className={classes.cellWrap} data-state={colState(s.id)}>
                  <div className={classes.cell} data-vote={mine.votes[s.id] ?? 'no'}>
                    <VoteMark vote={mine.votes[s.id] ?? 'no'} />
                  </div>
                </Table.Td>
              ))}
            </Table.Tr>
          )}
          {others.map((r) => (
            <Table.Tr key={r.user.id}>
              <Table.Td className={classes.sticky}>
                <Person user={r.user} />
              </Table.Td>
              {event.slots.map((s) => (
                <Table.Td key={s.id} className={classes.cellWrap} data-state={colState(s.id)}>
                  <div className={classes.cell} data-vote={r.votes[s.id] ?? 'no'}>
                    <VoteMark vote={r.votes[s.id] ?? 'no'} />
                  </div>
                </Table.Td>
              ))}
            </Table.Tr>
          ))}
        </Table.Tbody>
        <Table.Tfoot>
          <Table.Tr>
            <Table.Td className={classes.sticky}>
              <Text fz="sm" fw={600}>
                {t('vote.total')}
              </Text>
            </Table.Td>
            {scores.map((sc) => (
              <Table.Td key={sc.slot.id} ta="center" data-state={colState(sc.slot.id)} className={classes.cellWrap}>
                <Group gap={4} justify="center" wrap="nowrap">
                  <Text fz="sm" fw={700} c="teal" data-testid={`yes-${sc.slot.id}`}>
                    {sc.yes}
                  </Text>
                  {sc.maybe > 0 && (
                    <Text fz="xs" c="yellow.7" fw={600}>
                      +{sc.maybe}
                    </Text>
                  )}
                </Group>
              </Table.Td>
            ))}
          </Table.Tr>
        </Table.Tfoot>
      </Table>
    </ScrollArea>
  );
}
