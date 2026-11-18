import { Alert, Badge, Box, Card, Group, SimpleGrid, Stack, Text, ThemeIcon, Title } from '@mantine/core';
import { IconCalendarPlus, IconCheck, IconClockExclamation, IconShare, IconUsers } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { GoogleButton } from '../components/GoogleButton';

const features = [
  { icon: IconCalendarPlus, key: 'propose' },
  { icon: IconShare, key: 'share' },
  { icon: IconUsers, key: 'agree' },
] as const;

const preview = [
  { name: 'Ada', votes: ['yes', 'yes', 'maybe'] },
  { name: 'Alan', votes: ['yes', 'no', 'yes'] },
  { name: 'Grace', votes: ['yes', 'maybe', 'no'] },
];

function PreviewCell({ vote }: { vote: string }) {
  const color = vote === 'yes' ? 'teal' : vote === 'maybe' ? 'yellow' : 'gray';
  return (
    <Box
      h={34}
      style={{
        borderRadius: 8,
        display: 'grid',
        placeItems: 'center',
        background: `var(--mantine-color-${color}-light)`,
        color: `var(--mantine-color-${color}-light-color)`,
      }}
    >
      {vote === 'yes' && <IconCheck size={16} />}
      {vote === 'maybe' && (
        <Text fz="xs" fw={700}>
          (✓)
        </Text>
      )}
    </Box>
  );
}

export function Landing({ next, expired }: { next?: string; expired?: boolean }) {
  const { t } = useTranslation();
  return (
    <Stack gap={64} py={{ base: 'md', sm: 48 }}>
      {expired && (
        <Alert color="orange" variant="light" icon={<IconClockExclamation />} title={t('landing.expiredTitle')}>
          {t('landing.expiredText')}
        </Alert>
      )}
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing={48} style={{ alignItems: 'center' }}>
        <Stack gap="lg">
          <Badge variant="light" size="lg" w="fit-content">
            {t('landing.badge')}
          </Badge>
          <Title order={1} fz={{ base: 36, sm: 52 }} lh={1.1} style={{ letterSpacing: '-0.03em' }}>
            {t('landing.titleStart')}{' '}
            <Text span inherit variant="gradient">
              {t('landing.titleHighlight')}
            </Text>
          </Title>
          <Text fz="lg" c="dimmed" maw={480}>
            {t('landing.subtitle')}
          </Text>
          <Group>
            <GoogleButton size="lg" next={next} />
          </Group>
          {next && !expired && (
            <Text fz="sm" c="dimmed">
              {t('landing.signInToView')}
            </Text>
          )}
        </Stack>
        <Card shadow="xl" p="lg" aria-hidden="true">
          <Text fw={700}>{t('landing.preview.title')}</Text>
          <Text fz="sm" c="dimmed" mb="md">
            {t('landing.preview.meta')}
          </Text>
          <SimpleGrid cols={4} spacing={8} verticalSpacing={8}>
            <div />
            {(t('landing.preview.days', { returnObjects: true }) as string[]).map((d, i) => (
              <Text key={d} ta="center" fz="xs" fw={600} c={i === 0 ? 'teal' : 'dimmed'}>
                {d}
              </Text>
            ))}
            {preview.map((p) => [
              <Text key={p.name} fz="sm" fw={500} style={{ alignSelf: 'center' }}>
                {p.name}
              </Text>,
              ...p.votes.map((v, i) => <PreviewCell key={p.name + i} vote={v} />),
            ])}
          </SimpleGrid>
        </Card>
      </SimpleGrid>
      <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="lg">
        {features.map((f) => (
          <Card key={f.key} p="lg">
            <ThemeIcon variant="light" size={42} radius="md" mb="md">
              <f.icon size={22} />
            </ThemeIcon>
            <Text fw={700} mb={4}>
              {t(`landing.features.${f.key}Title`)}
            </Text>
            <Text fz="sm" c="dimmed">
              {t(`landing.features.${f.key}Text`)}
            </Text>
          </Card>
        ))}
      </SimpleGrid>
    </Stack>
  );
}
