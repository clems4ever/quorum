import {
  Alert,
  Anchor,
  Button,
  Card,
  Center,
  Group,
  Loader,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconArrowLeft, IconCalendar, IconClock, IconMapPin, IconRestore } from '@tabler/icons-react';
import { useEffect, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { CoverImagePicker } from '../components/CoverImagePicker';
import { DaySlotPicker } from '../components/DaySlotPicker';
import { HourSlotPicker } from '../components/HourSlotPicker';
import { errorMessage, useApi } from '../lib/api';
import { useAuth } from '../lib/auth-context';
import { CENTER, type CoverChange } from '../lib/cover';
import type { Cover, EventInput, Focus, Mode, SlotInput } from '../lib/types';

const draftKey = (id?: string) => `quorum:draft:${id ?? 'new'}`;

function readDraft(id?: string): EventInput | null {
  try {
    return JSON.parse(sessionStorage.getItem(draftKey(id)) ?? 'null') as EventInput | null;
  } catch {
    return null;
  }
}

const emptyEvent: EventInput = { title: '', description: '', location: '', mode: 'days', slots: [] };

/** Load error used when someone else's event is opened for editing. */
const notOrganizer = Symbol('not organizer');

export function EventEditor({ today }: { today?: Date }) {
  const { t } = useTranslation();
  const { id } = useParams();
  const api = useApi();
  const { user } = useAuth();
  const navigate = useNavigate();
  // A new event starts from its saved draft; an existing one once loaded.
  const [initial] = useState(() => (id ? emptyEvent : (readDraft() ?? emptyEvent)));
  const [title, setTitle] = useState(initial.title);
  const [description, setDescription] = useState(initial.description);
  const [location, setLocation] = useState(initial.location);
  const [mode, setMode] = useState<Mode>(initial.mode);
  const [slots, setSlots] = useState<SlotInput[]>(initial.slots);
  const [loading, setLoading] = useState(!!id);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [saving, setSaving] = useState(false);
  const [touched, setTouched] = useState(false);
  // dirty: the user changed something that is worth keeping as a draft.
  const [dirty, setDirty] = useState(false);
  const [restored, setRestored] = useState(initial !== emptyEvent);
  // Images are not part of drafts: files cannot be kept in session storage.
  const [cover, setCover] = useState<CoverChange>({ kind: 'keep' });
  const [currentCover, setCurrentCover] = useState<Cover | undefined>();
  const [focus, setFocus] = useState<Focus>(CENTER);
  const [focusChanged, setFocusChanged] = useState(false);
  const [reload, setReload] = useState(0);

  const apply = (e: EventInput) => {
    setTitle(e.title);
    setDescription(e.description);
    setLocation(e.location);
    setMode(e.mode);
    setSlots(e.slots);
  };
  const edit =
    <T,>(set: (v: T) => void) =>
    (v: T) => {
      set(v);
      setDirty(true);
    };

  useEffect(() => {
    if (!id) return;
    let alive = true;
    api
      .getEvent(id)
      .then((e) => {
        if (!alive) return;
        if (e.owner.id !== user?.id) {
          setLoadError(notOrganizer);
          return;
        }
        const draft = readDraft(id);
        apply(draft ?? e);
        setRestored(!!draft);
        setCurrentCover(e.cover);
        setFocus(e.cover?.focus ?? CENTER);
      })
      .catch((e: unknown) => alive && setLoadError(e))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [api, id, user?.id, reload]);

  useEffect(() => {
    if (dirty) {
      sessionStorage.setItem(draftKey(id), JSON.stringify({ title, description, location, mode, slots }));
    }
  }, [dirty, id, title, description, location, mode, slots]);

  const discardDraft = () => {
    sessionStorage.removeItem(draftKey(id));
    setDirty(false);
    setRestored(false);
    setTouched(false);
    if (id) {
      setReload(reload + 1);
    } else {
      apply(emptyEvent);
    }
  };

  const titleError = touched && !title.trim() ? t('editor.titleRequired') : null;
  const slotsError = touched && slots.length === 0 ? t('editor.slotsRequired') : null;

  // The event itself is saved first, so a failed upload only loses the image.
  const saveCover = async (eventId: string) => {
    try {
      if (cover.kind === 'set') await api.uploadImage(eventId, cover.file, focus);
      if (cover.kind === 'remove') await api.deleteImage(eventId);
      // Only the position of the existing image changed.
      if (cover.kind === 'keep' && currentCover && focusChanged) await api.setImageFocus(eventId, focus);
    } catch (err) {
      notifications.show({ color: 'orange', title: t('cover.uploadFailed'), message: errorMessage(err) });
    }
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (!title.trim() || slots.length === 0) return;
    setSaving(true);
    try {
      const input = { title, description, location, mode, slots };
      const saved = id ? await api.updateEvent(id, input) : await api.createEvent(input);
      sessionStorage.removeItem(draftKey(id));
      await saveCover(saved.id);
      notifications.show({
        color: 'teal',
        title: id ? t('editor.updated') : t('editor.created'),
        message: t('editor.shareHint'),
      });
      await navigate(`/e/${saved.id}`);
    } catch (err) {
      notifications.show({ color: 'red', title: t('editor.saveFailed'), message: errorMessage(err) });
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <Center py={80}>
        <Loader />
      </Center>
    );
  }
  if (loadError !== null) {
    return (
      <Alert color="red" title={t('editor.cannotEdit')}>
        {loadError === notOrganizer ? t('editor.onlyOrganizer') : errorMessage(loadError)}
      </Alert>
    );
  }

  return (
    <form onSubmit={(e) => void submit(e)} noValidate>
      <Stack gap="xl">
        <div>
          <Anchor component={Link} to={id ? `/e/${id}` : '/'} fz="sm" c="dimmed">
            <Group gap={4}>
              <IconArrowLeft size={14} /> {t('common.back')}
            </Group>
          </Anchor>
          <Title order={2} mt={4}>
            {id ? t('editor.titleEdit') : t('editor.titleNew')}
          </Title>
          <Text c="dimmed">{t('editor.subtitle')}</Text>
        </div>
        {restored && (
          <Alert color="blue" variant="light" icon={<IconRestore />} title={t('editor.draftRestored')}>
            <Group justify="space-between">
              <Text fz="sm">{t('editor.draftRestoredText')}</Text>
              <Button size="xs" variant="subtle" onClick={discardDraft}>
                {t('editor.discardDraft')}
              </Button>
            </Group>
          </Alert>
        )}
        <Card p="xl">
          <Stack>
            <Text fw={700}>{t('editor.details')}</Text>
            <SimpleGrid cols={{ base: 1, sm: 2 }}>
              <TextInput
                label={t('editor.title')}
                placeholder={t('editor.titlePlaceholder')}
                required
                maxLength={120}
                value={title}
                error={titleError}
                onChange={(e) => edit(setTitle)(e.currentTarget.value)}
              />
              <TextInput
                label={t('editor.location')}
                placeholder={t('editor.locationPlaceholder')}
                leftSection={<IconMapPin size={16} />}
                maxLength={200}
                value={location}
                onChange={(e) => edit(setLocation)(e.currentTarget.value)}
              />
            </SimpleGrid>
            <Textarea
              label={t('editor.description')}
              placeholder={t('editor.descriptionPlaceholder')}
              autosize
              minRows={3}
              maxRows={8}
              maxLength={2000}
              value={description}
              onChange={(e) => edit(setDescription)(e.currentTarget.value)}
            />
            <div>
              <Text fz="sm" fw={500} mb={4}>
                {t('cover.title')}
              </Text>
              <CoverImagePicker
                currentUrl={currentCover?.url}
                value={cover}
                onChange={setCover}
                focus={focus}
                onFocusChange={(f) => {
                  setFocus(f);
                  setFocusChanged(true);
                }}
              />
            </div>
          </Stack>
        </Card>
        <Card p="xl">
          <Stack>
            <Group justify="space-between">
              <div>
                <Text fw={700}>{t('editor.options')}</Text>
                <Text fz="sm" c="dimmed">
                  {mode === 'days' ? t('editor.pickDays') : t('editor.pickSlots')}
                </Text>
              </div>
              <SegmentedControl
                value={mode}
                disabled={!!id}
                aria-label={t('editor.slotType')}
                onChange={(v) => {
                  setMode(v);
                  edit(setSlots)([]);
                }}
                data={[
                  {
                    value: 'days',
                    label: (
                      <Group gap={6} wrap="nowrap">
                        <IconCalendar size={16} />
                        {t('editor.wholeDays')}
                      </Group>
                    ),
                  },
                  {
                    value: 'hours',
                    label: (
                      <Group gap={6} wrap="nowrap">
                        <IconClock size={16} />
                        {t('editor.timeSlots')}
                      </Group>
                    ),
                  },
                ]}
              />
            </Group>
            {mode === 'days' ? (
              <DaySlotPicker value={slots} onChange={edit(setSlots)} today={today} />
            ) : (
              <HourSlotPicker value={slots} onChange={edit(setSlots)} today={today} />
            )}
            {slotsError && (
              <Text c="red" fz="sm" role="alert">
                {slotsError}
              </Text>
            )}
          </Stack>
        </Card>
        <Group justify="flex-end">
          <Button
            variant="default"
            component={Link}
            to={id ? `/e/${id}` : '/'}
            onClick={() => sessionStorage.removeItem(draftKey(id))}
          >
            {t('common.cancel')}
          </Button>
          <Button type="submit" loading={saving} size="md">
            {id ? t('editor.save') : t('editor.create')}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
