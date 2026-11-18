import { Group, Text, ThemeIcon } from '@mantine/core';
import { IconCalendarCheck } from '@tabler/icons-react';

export function Logo() {
  return (
    <Group gap={8} wrap="nowrap">
      <ThemeIcon variant="gradient" size={34} radius="md">
        <IconCalendarCheck size={20} stroke={2} />
      </ThemeIcon>
      <Text fw={800} fz={20} style={{ letterSpacing: '-0.02em' }}>
        Quorum
      </Text>
    </Group>
  );
}
