import { ActionIcon, Tooltip, useComputedColorScheme, useMantineColorScheme } from '@mantine/core';
import { IconMoon, IconSun } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';

export function ThemeToggle() {
  const { t } = useTranslation();
  const { setColorScheme } = useMantineColorScheme();
  const computed = useComputedColorScheme('light', { getInitialValueInEffect: false });
  const next = computed === 'dark' ? 'light' : 'dark';
  const label = t(`nav.switchTheme_${next}`);
  return (
    <Tooltip label={label}>
      <ActionIcon variant="default" size="lg" radius="md" aria-label={label} onClick={() => setColorScheme(next)}>
        {computed === 'dark' ? <IconSun size={18} /> : <IconMoon size={18} />}
      </ActionIcon>
    </Tooltip>
  );
}
