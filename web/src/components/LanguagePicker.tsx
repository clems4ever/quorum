import { Button, Menu } from '@mantine/core';
import { IconCheck, IconLanguage } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { chooseLanguage, languages } from '../i18n';

export function LanguagePicker() {
  const { t, i18n } = useTranslation();
  const current = i18n.resolvedLanguage;
  return (
    <Menu position="bottom-end" shadow="md" width={160}>
      <Menu.Target>
        <Button
          variant="default"
          px={10}
          leftSection={<IconLanguage size={16} />}
          aria-label={t('nav.language')}
          tt="uppercase"
        >
          {current}
        </Button>
      </Menu.Target>
      <Menu.Dropdown>
        {languages.map((l) => (
          <Menu.Item
            key={l.code}
            lang={l.code}
            onClick={() => void chooseLanguage(l.code)}
            rightSection={l.code === current ? <IconCheck size={14} /> : null}
          >
            {l.label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  );
}
