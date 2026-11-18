import {
  ActionIcon,
  Alert,
  Anchor,
  Avatar,
  Badge,
  Button,
  Card,
  Center,
  CopyButton,
  Group,
  Image,
  Loader,
  Menu,
  Modal,
  Radio,
  Stack,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import {
  IconArrowLeft,
  IconCalendarCheck,
  IconCheck,
  IconDots,
  IconEdit,
  IconLink,
  IconLock,
  IconLockOpen,
  IconMapPin,
  IconTrash,
} from '@tabler/icons-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { VoteGrid, VoteMark } from '../components/VoteGrid';
import { ApiError, errorMessage, useApi } from '../lib/api';
import { useAuth } from '../lib/auth-context';
import { bestSlotIds, formatSlot, scoreSlots } from '../lib/slots';
import { nextVote } from '../lib/votes';
import { objectPosition } from '../lib/cover';
import type { EventDetail, VoteOrNo } from '../lib/types';

function draftFrom(event: EventDetail, userId: string): Record<string, VoteOrNo> {
  const votes = event.responses.find((r) => r.user.id === userId)?.votes ?? {};
  return Object.fromEntries(event.slots.map((s) => [s.id, votes[s.id] ?? 'no']));
}

interface CloseProps {
  event: EventDetail;
  onClose: () => void;
  onConfirm: (finalSlotId?: string) => Promise<void>;
}

function CloseModal({ opened, ...props }: CloseProps & { opened: boolean }) {
  const { t } = useTranslation();
  // The form is mounted each time the modal opens, so it starts from fresh state.
  return (
    <Modal
      opened={opened}
      onClose={props.onClose}
      title={<Text fw={700}>{t('closeModal.title')}</Text>}
      size="lg"
      centered
    >
      <CloseForm {...props} />
    </Modal>
  );
}

function CloseForm({ event, onClose, onConfirm }: CloseProps) {
  const { t } = useTranslation();
  const ranked = useMemo(() => scoreSlots(event).sort((a, b) => b.score - a.score), [event]);
  // Preselect the most popular option, if any.
  const [choice, setChoice] = useState(() => (ranked[0]?.score ? ranked[0].slot.id : ''));
  const [busy, setBusy] = useState(false);

  return (
    <Stack>
      <Text fz="sm" c="dimmed">
        {t('closeModal.text')}
      </Text>
      <Radio.Group value={choice} onChange={setChoice} aria-label={t('closeModal.finalOption')}>
        <Stack gap="xs">
          {ranked.map((s) => (
            <Radio.Card key={s.slot.id} value={s.slot.id} p="sm" radius="md">
              <Group wrap="nowrap">
                <Radio.Indicator />
                <Text fz="sm" fw={500} style={{ flex: 1 }}>
                  {formatSlot(s.slot, event.mode)}
                </Text>
                <Badge variant="light" color="teal">
                  {t('closeModal.yesCount', { count: s.yes })}
                </Badge>
                {s.maybe > 0 && (
                  <Badge variant="light" color="yellow">
                    {t('closeModal.maybeCount', { count: s.maybe })}
                  </Badge>
                )}
              </Group>
            </Radio.Card>
          ))}
          <Radio.Card value="" p="sm" radius="md">
            <Group>
              <Radio.Indicator />
              <Text fz="sm">{t('closeModal.noDate')}</Text>
            </Group>
          </Radio.Card>
        </Stack>
      </Radio.Group>
      <Group justify="flex-end">
        <Button variant="default" onClick={onClose}>
          {t('common.cancel')}
        </Button>
        <Button
          loading={busy}
          onClick={() => {
            setBusy(true);
            void onConfirm(choice || undefined).finally(() => setBusy(false));
          }}
        >
          {t('closeModal.submit')}
        </Button>
      </Group>
    </Stack>
  );
}

export function EventPage() {
  const { id = '' } = useParams();
  // Remount for every event so no state leaks from the previous one.
  return <EventView key={id} id={id} />;
}

function EventView({ id }: { id: string }) {
  const { t } = useTranslation();
  const api = useApi();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [event, setEvent] = useState<EventDetail | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [draft, setDraft] = useState<Record<string, VoteOrNo>>({});
  const [saving, setSaving] = useState(false);
  const [closing, setClosing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const me = user!;

  const load = (e: EventDetail) => {
    setEvent(e);
    setDraft(draftFrom(e, me.id));
  };

  useEffect(() => {
    let alive = true;
    api
      .getEvent(id)
      .then((e) => {
        if (!alive) return;
        setEvent(e);
        setDraft(draftFrom(e, me.id));
      })
      .catch((e: unknown) => alive && setError(e));
    return () => {
      alive = false;
    };
  }, [api, id, me.id]);

  const run = async (action: () => Promise<EventDetail>, success: string) => {
    try {
      load(await action());
      notifications.show({ color: 'teal', message: success });
      return true;
    } catch (e) {
      notifications.show({ color: 'red', title: t('common.somethingWrong'), message: errorMessage(e) });
      return false;
    }
  };

  if (error !== null) {
    return (
      <Stack align="center" py={60}>
        <Alert color="red" title={t('event.cannotOpen')} maw={480} w="100%">
          {error instanceof ApiError && error.status === 404 ? t('event.notFound') : errorMessage(error)}
        </Alert>
        <Button component={Link} to="/" variant="default">
          {t('event.backToEvents')}
        </Button>
      </Stack>
    );
  }
  if (!event) {
    return (
      <Center py={80}>
        <Loader />
      </Center>
    );
  }

  const isOwner = event.owner.id === me.id;
  const hasResponded = event.responses.some((r) => r.user.id === me.id);
  const saved = draftFrom(event, me.id);
  const dirty = event.slots.some((s) => saved[s.id] !== draft[s.id]);
  const scores = scoreSlots(event);
  const best = bestSlotIds(scores);
  const finalSlot = event.slots.find((s) => s.id === event.finalSlotId);
  const shareUrl = `${window.location.origin}/e/${event.id}`;

  const remove = async () => {
    try {
      await api.deleteEvent(event.id);
      notifications.show({ color: 'teal', message: t('event.deleted') });
      await navigate('/');
    } catch (e) {
      notifications.show({ color: 'red', title: t('event.deleteFailed'), message: errorMessage(e) });
    }
  };

  const save = async () => {
    setSaving(true);
    await run(() => api.respond(event.id, draft), t('event.votesSaved'));
    setSaving(false);
  };

  return (
    <Stack gap="lg">
      <Anchor component={Link} to="/" fz="sm" c="dimmed">
        <Group gap={4}>
          <IconArrowLeft size={14} /> {t('event.myEvents')}
        </Group>
      </Anchor>

      <Card padding="xl">
        {event.cover && (
          <Card.Section mb="lg">
            <Image
              src={event.cover.url}
              alt={event.title}
              h={260}
              fit="cover"
              style={{ objectPosition: objectPosition(event.cover.focus) }}
            />
          </Card.Section>
        )}
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <Stack gap={6}>
            <Group gap="xs">
              <Badge variant="dot" color={event.closed ? 'gray' : 'teal'}>
                {event.closed ? t('event.closed') : t('event.openForVotes')}
              </Badge>
              <Badge variant="light">{event.mode === 'days' ? t('editor.wholeDays') : t('editor.timeSlots')}</Badge>
            </Group>
            <Title order={2}>{event.title}</Title>
            <Group gap="lg">
              <Group gap={6}>
                <Avatar
                  src={event.owner.picture || null}
                  name={event.owner.name}
                  color="initials"
                  size={22}
                  radius="xl"
                />
                <Text fz="sm" c="dimmed">
                  {isOwner ? t('event.organizedByYou') : t('event.organizedBy', { name: event.owner.name })}
                </Text>
              </Group>
              {event.location && (
                <Group gap={4} c="dimmed">
                  <IconMapPin size={16} />
                  <Text fz="sm">{event.location}</Text>
                </Group>
              )}
            </Group>
            {event.description && (
              <Text fz="sm" mt="xs" style={{ whiteSpace: 'pre-wrap' }}>
                {event.description}
              </Text>
            )}
          </Stack>
          <Group gap="xs" wrap="nowrap">
            <CopyButton value={shareUrl}>
              {({ copied, copy }) => (
                <Button
                  variant="light"
                  color={copied ? 'teal' : undefined}
                  leftSection={copied ? <IconCheck size={16} /> : <IconLink size={16} />}
                  onClick={copy}
                >
                  {copied ? t('event.linkCopied') : t('event.share')}
                </Button>
              )}
            </CopyButton>
            {isOwner && (
              <Menu position="bottom-end" shadow="md" width={200}>
                <Menu.Target>
                  <ActionIcon variant="default" size="lg" aria-label={t('event.actions')}>
                    <IconDots size={18} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item leftSection={<IconEdit size={16} />} component={Link} to={`/e/${event.id}/edit`}>
                    {t('event.edit')}
                  </Menu.Item>
                  {event.closed ? (
                    <Menu.Item
                      leftSection={<IconLockOpen size={16} />}
                      onClick={() => void run(() => api.reopenEvent(event.id), t('event.reopened'))}
                    >
                      {t('event.reopen')}
                    </Menu.Item>
                  ) : (
                    <Menu.Item leftSection={<IconLock size={16} />} onClick={() => setClosing(true)}>
                      {t('event.close')}
                    </Menu.Item>
                  )}
                  <Menu.Divider />
                  <Menu.Item color="red" leftSection={<IconTrash size={16} />} onClick={() => setDeleting(true)}>
                    {t('event.delete')}
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            )}
          </Group>
        </Group>
      </Card>

      {finalSlot && (
        <Alert color="teal" variant="light" icon={<IconCalendarCheck />} title={t('event.dateSet')}>
          {formatSlot(finalSlot, event.mode)}
        </Alert>
      )}
      {event.closed && !finalSlot && (
        <Alert color="gray" variant="light" icon={<IconLock />}>
          {t('event.closedNotice')}
        </Alert>
      )}

      <Card p="lg">
        <Group justify="space-between" mb="md">
          <div>
            <Text fw={700}>{t('event.availability')}</Text>
            {!event.closed && (
              <Text fz="sm" c="dimmed">
                {t('event.cycleHint')}
              </Text>
            )}
          </div>
          <Group gap="md" fz="xs" c="dimmed">
            {(['yes', 'maybe', 'no'] as const).map((v) => (
              <Group key={v} gap={4}>
                <VoteMark vote={v} />
                <Text fz="xs">{t(`vote.${v}`)}</Text>
              </Group>
            ))}
          </Group>
        </Group>
        <VoteGrid
          event={event}
          me={me}
          draft={event.closed ? undefined : draft}
          onCycle={(slotId) => setDraft({ ...draft, [slotId]: nextVote(draft[slotId] ?? 'no') })}
        />
        {!event.closed && (
          <Group justify="space-between" mt="md">
            <Group gap="xs">
              <Button
                variant="subtle"
                size="xs"
                onClick={() => setDraft(Object.fromEntries(event.slots.map((s) => [s.id, 'yes'])))}
              >
                {t('event.yesToAll')}
              </Button>
              <Button variant="subtle" size="xs" color="gray" onClick={() => setDraft(saved)} disabled={!dirty}>
                {t('event.reset')}
              </Button>
              {hasResponded && (
                <Button
                  variant="subtle"
                  size="xs"
                  color="red"
                  onClick={() => void run(() => api.withdraw(event.id), t('event.responseRemoved'))}
                >
                  {t('event.withdraw')}
                </Button>
              )}
            </Group>
            <Button onClick={() => void save()} loading={saving} disabled={hasResponded && !dirty}>
              {hasResponded ? t('event.updateVotes') : t('event.saveVotes')}
            </Button>
          </Group>
        )}
      </Card>

      {best.size > 0 && !finalSlot && (
        <Text fz="sm" c="dimmed" ta="center">
          {t('event.mostPopular')}{' '}
          <Text span fw={600} c="var(--mantine-color-text)">
            {scores
              .filter((s) => best.has(s.slot.id))
              .map((s) => formatSlot(s.slot, event.mode))
              .join(' · ')}
          </Text>
        </Text>
      )}

      {isOwner && (
        <>
          <CloseModal
            event={event}
            opened={closing}
            onClose={() => setClosing(false)}
            onConfirm={async (finalSlotId) => {
              if (await run(() => api.closeEvent(event.id, finalSlotId), t('event.closedToast'))) setClosing(false);
            }}
          />
          <Modal
            opened={deleting}
            onClose={() => setDeleting(false)}
            title={<Text fw={700}>{t('event.deleteTitle')}</Text>}
            centered
          >
            <Text fz="sm">{t('event.deleteConfirm', { title: event.title })}</Text>
            <Group justify="flex-end" mt="lg">
              <Button variant="default" onClick={() => setDeleting(false)}>
                {t('common.cancel')}
              </Button>
              <Tooltip label={t('event.deleteTooltip')}>
                <Button color="red" onClick={() => void remove()}>
                  {t('event.deleteButton')}
                </Button>
              </Tooltip>
            </Group>
          </Modal>
        </>
      )}
    </Stack>
  );
}
