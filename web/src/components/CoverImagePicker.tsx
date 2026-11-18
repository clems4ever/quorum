import { Box, Button, FileButton, Group, Image, Stack, Text, ThemeIcon } from '@mantine/core';
import { Dropzone, type FileRejection } from '@mantine/dropzone';
import { IconArrowsMove, IconPhoto, IconReplace, IconTrash } from '@tabler/icons-react';
import { useEffect, useMemo, useRef, useState, type PointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import {
  CENTER,
  checkImage,
  dragFocus,
  IMAGE_TYPES,
  MAX_IMAGE_BYTES,
  nudgeFocus,
  objectPosition,
  type CoverChange,
} from '../lib/cover';
import type { Focus } from '../lib/types';
import classes from './CoverImagePicker.module.css';

interface Props {
  /** URL of the image currently stored for the event, if any. */
  currentUrl?: string;
  value: CoverChange;
  onChange: (change: CoverChange) => void;
  focus: Focus;
  onFocusChange: (focus: Focus) => void;
}

/** A temporary URL showing a local file, released when no longer needed. */
function useObjectUrl(file: File | null): string | undefined {
  const url = useMemo(() => (file ? URL.createObjectURL(file) : undefined), [file]);
  useEffect(
    () => () => {
      if (url) URL.revokeObjectURL(url);
    },
    [url],
  );
  return url;
}

interface Drag {
  pointerId: number;
  x: number;
  y: number;
  focus: Focus;
  box: { width: number; height: number };
}

export function CoverImagePicker({ currentUrl, value, onChange, focus, onFocusChange }: Props) {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const [natural, setNatural] = useState<{ width: number; height: number } | null>(null);
  const drag = useRef<Drag | null>(null);
  const preview = useObjectUrl(value.kind === 'set' ? value.file : null);
  const shown = value.kind === 'set' ? preview : value.kind === 'keep' ? currentUrl : undefined;

  const choose = (file: File | null) => {
    if (!file) return;
    const problem = checkImage(file);
    if (problem) {
      setError(t(`cover.${problem}`));
      return;
    }
    setError(null);
    setNatural(null);
    onChange({ kind: 'set', file });
    onFocusChange(CENTER);
  };

  const reject = (rejections: FileRejection[]) => {
    const tooLarge = rejections.some((r) => r.errors.some((e) => e.code === 'file-too-large'));
    setError(tooLarge ? t('cover.tooLarge') : t('cover.invalidType'));
  };

  const startDrag = (e: PointerEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = {
      pointerId: e.pointerId,
      x: e.clientX,
      y: e.clientY,
      focus,
      box: { width: rect.width, height: rect.height },
    };
  };

  const moveDrag = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current;
    if (!d || d.pointerId !== e.pointerId || !natural) return;
    onFocusChange(dragFocus(d.focus, e.clientX - d.x, e.clientY - d.y, d.box, natural));
  };

  const endDrag = () => {
    drag.current = null;
  };

  if (!shown) {
    return (
      <Stack gap="xs">
        <Dropzone
          onDrop={(files) => choose(files[0])}
          onReject={reject}
          accept={IMAGE_TYPES}
          maxSize={MAX_IMAGE_BYTES}
          multiple={false}
          radius="md"
          p="xl"
          aria-label={t('cover.drop')}
        >
          <Stack align="center" gap={6} style={{ pointerEvents: 'none' }}>
            <ThemeIcon variant="light" size={48} radius="xl">
              <IconPhoto size={26} />
            </ThemeIcon>
            <Text fw={500}>{t('cover.drop')}</Text>
            <Text fz="xs" c="dimmed">
              {t('cover.hint')}
            </Text>
          </Stack>
        </Dropzone>
        {error && (
          <Text c="red" fz="sm" role="alert">
            {error}
          </Text>
        )}
      </Stack>
    );
  }

  return (
    <Stack gap="xs">
      <Box
        className={classes.frame}
        tabIndex={0}
        role="group"
        aria-label={t('cover.repositionLabel')}
        data-testid="cover-frame"
        onPointerDown={startDrag}
        onPointerMove={moveDrag}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onKeyDown={(e) => {
          const next = nudgeFocus(focus, e.key, e.shiftKey ? 0.2 : 0.05);
          if (next) {
            e.preventDefault();
            onFocusChange(next);
          }
        }}
      >
        <Image
          src={shown}
          alt={t('cover.title')}
          h={200}
          fit="cover"
          draggable={false}
          style={{ objectPosition: objectPosition(focus) }}
          onLoad={(e) => setNatural({ width: e.currentTarget.naturalWidth, height: e.currentTarget.naturalHeight })}
        />
        <Group gap={6} className={classes.hint}>
          <IconArrowsMove size={14} />
          {t('cover.reposition')}
        </Group>
      </Box>
      <Group justify="flex-end" gap="xs">
        <FileButton onChange={choose} accept={IMAGE_TYPES.join(',')}>
          {(props) => (
            <Button {...props} variant="default" size="xs" leftSection={<IconReplace size={14} />}>
              {t('cover.replace')}
            </Button>
          )}
        </FileButton>
        <Button
          variant="subtle"
          color="red"
          size="xs"
          leftSection={<IconTrash size={14} />}
          onClick={() => {
            setError(null);
            onChange(currentUrl ? { kind: 'remove' } : { kind: 'keep' });
          }}
        >
          {t('cover.remove')}
        </Button>
      </Group>
      {error && (
        <Text c="red" fz="sm" role="alert">
          {error}
        </Text>
      )}
    </Stack>
  );
}
