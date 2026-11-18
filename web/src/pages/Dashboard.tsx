import {
  Alert,
  Avatar,
  Badge,
  Box,
  Button,
  Card,
  Group,
  Image,
  SegmentedControl,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from '@mantine/core';
import { IconCalendarEvent, IconCalendarPlus, IconClock, IconSearch, IconUsers } from '@tabler/icons-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { errorMessage, useApi } from '../lib/api';
import { useAuth } from '../lib/auth-context';
import { objectPosition } from '../lib/cover';
import { formatRange } from '../lib/slots';
import type { EventSummary } from '../lib/types';

type Filter = 'all' | 'owner' | 'participant';

function EventCard({ e }: { e: EventSummary }) {
  const { t } = useTranslation();
  return (
    <Card component={Link} to={`/e/${e.id}`} padding="lg" className="hover-lift" aria-label={e.title}>
      <Card.Section mb="md">
        {e.cover ? (
          <Image
            src={e.cover.url}
            alt=""
            h={120}
            fit="cover"
            style={{ objectPosition: objectPosition(e.cover.focus) }}
          />
        ) : (
          <Box h={120} className="cover-placeholder" data-testid="cover-placeholder">
            <IconCalendarEvent size={36} stroke={1.5} />
          </Box>
        )}
      </Card.Section>
      <Group justify="space-between" mb="xs" wrap="nowrap">
        <Badge variant="light" color={e.role === 'owner' ? 'indigo' : 'grape'}>
          {e.role === 'owner' ? t('dashboard.organizer') : t('dashboard.participant')}
        </Badge>
        <Badge variant="dot" color={e.closed ? 'gray' : 'teal'}>
          {e.closed ? t('dashboard.closed') : t('dashboard.open')}
        </Badge>
      </Group>
      <Text fw={700} fz="lg" lineClamp={1}>
        {e.title}
      </Text>
      <Text fz="sm" c="dimmed" lineClamp={1} mb="md">
        {e.location || ' '}
      </Text>
      <Stack gap={6}>
        <Group gap={6} c="dimmed" fz="sm">
          <IconCalendarEvent size={16} />
          <Text fz="sm">{formatRange(e.firstStart, e.lastEnd, e.mode)}</Text>
        </Group>
        <Group gap={6} c="dimmed">
          <IconClock size={16} />
          <Text fz="sm">
            {t(e.mode === 'days' ? 'dashboard.dayOptions' : 'dashboard.timeOptions', { count: e.slotCount })}
          </Text>
        </Group>
        <Group gap={6} c="dimmed">
          <IconUsers size={16} />
          <Text fz="sm">{t('dashboard.responses', { count: e.responseCount })}</Text>
        </Group>
      </Stack>
      <Group gap={8} mt="md" pt="sm" style={{ borderTop: '1px solid var(--mantine-color-default-border)' }}>
        <Avatar src={e.owner.picture || null} name={e.owner.name} color="initials" size={24} radius="xl" />
        <Text fz="xs" c="dimmed">
          {t('dashboard.by', { name: e.owner.name })}
        </Text>
      </Group>
    </Card>
  );
}

function EmptyState() {
  const { t } = useTranslation();
  return (
    <Card p={48} style={{ textAlign: 'center' }}>
      <Stack align="center" gap="sm">
        <ThemeIcon variant="light" size={64} radius="xl">
          <IconCalendarPlus size={32} />
        </ThemeIcon>
        <Title order={3}>{t('dashboard.emptyTitle')}</Title>
        <Text c="dimmed" maw={420}>
          {t('dashboard.emptyText')}
        </Text>
        <Button component={Link} to="/new" mt="sm" leftSection={<IconCalendarPlus size={18} />}>
          {t('dashboard.createEvent')}
        </Button>
      </Stack>
    </Card>
  );
}

export function Dashboard() {
  const { t } = useTranslation();
  const api = useApi();
  const { user } = useAuth();
  const [events, setEvents] = useState<EventSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');

  useEffect(() => {
    api
      .listEvents()
      .then(setEvents)
      .catch((e) => setError(errorMessage(e)));
  }, [api]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (events ?? []).filter(
      (e) =>
        (filter === 'all' || e.role === filter) &&
        (!q || e.title.toLowerCase().includes(q) || e.location.toLowerCase().includes(q)),
    );
  }, [events, filter, query]);

  const firstName = user?.name.split(' ')[0];

  return (
    <Stack gap="xl">
      <Group justify="space-between" align="flex-end">
        <div>
          <Title order={2}>
            {firstName ? t('dashboard.welcomeName', { name: firstName }) : t('dashboard.welcome')}
          </Title>
          <Text c="dimmed">{t('dashboard.subtitle')}</Text>
        </div>
        <Button component={Link} to="/new" leftSection={<IconCalendarPlus size={18} />}>
          {t('nav.newEvent')}
        </Button>
      </Group>
      {error && (
        <Alert color="red" title={t('dashboard.loadError')}>
          {error}
        </Alert>
      )}
      {!events && !error && (
        <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} data-testid="loading">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} h={220} radius="lg" />
          ))}
        </SimpleGrid>
      )}
      {events && events.length === 0 && <EmptyState />}
      {events && events.length > 0 && (
        <>
          <Group justify="space-between">
            <SegmentedControl
              value={filter}
              onChange={setFilter}
              data={[
                { label: t('dashboard.filterAll', { count: events.length }), value: 'all' },
                { label: t('dashboard.filterOwner'), value: 'owner' },
                { label: t('dashboard.filterParticipant'), value: 'participant' },
              ]}
            />
            <TextInput
              placeholder={t('dashboard.search')}
              aria-label={t('dashboard.search')}
              leftSection={<IconSearch size={16} />}
              value={query}
              onChange={(e) => setQuery(e.currentTarget.value)}
              w={{ base: '100%', xs: 260 }}
            />
          </Group>
          {visible.length === 0 ? (
            <Text c="dimmed" ta="center" py="xl">
              {t('dashboard.noMatch')}
            </Text>
          ) : (
            <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="lg">
              {visible.map((e) => (
                <EventCard key={e.id} e={e} />
              ))}
            </SimpleGrid>
          )}
        </>
      )}
    </Stack>
  );
}
