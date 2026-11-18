import { AppShell, Avatar, Button, Container, Group, Menu, Text, UnstyledButton } from '@mantine/core';
import { IconChevronDown, IconLogout, IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link, Outlet } from 'react-router-dom';
import { useAuth } from '../lib/auth-context';
import { LanguagePicker } from './LanguagePicker';
import { Logo } from './Logo';
import { ThemeToggle } from './ThemeToggle';

export function Layout() {
  const { t } = useTranslation();
  const { user, signOut } = useAuth();
  return (
    <AppShell header={{ height: 64 }} padding={0}>
      <AppShell.Header>
        <Container size="lg" h="100%">
          <Group h="100%" justify="space-between" wrap="nowrap">
            <UnstyledButton component={Link} to="/" aria-label={t('nav.home')}>
              <Logo />
            </UnstyledButton>
            <Group gap="sm" wrap="nowrap">
              {user && (
                <Button component={Link} to="/new" leftSection={<IconPlus size={16} />} visibleFrom="xs">
                  {t('nav.newEvent')}
                </Button>
              )}
              <LanguagePicker />
              <ThemeToggle />
              {user && (
                <Menu position="bottom-end" width={240} shadow="md">
                  <Menu.Target>
                    <UnstyledButton aria-label={t('nav.accountMenu')}>
                      <Group gap={6} wrap="nowrap">
                        <Avatar src={user.picture || null} name={user.name} color="initials" radius="xl" size={34} />
                        <IconChevronDown size={14} />
                      </Group>
                    </UnstyledButton>
                  </Menu.Target>
                  <Menu.Dropdown>
                    <Menu.Label>
                      <Text fz="sm" fw={600} c="var(--mantine-color-text)" truncate>
                        {user.name}
                      </Text>
                      <Text fz="xs" c="dimmed" truncate>
                        {user.email}
                      </Text>
                    </Menu.Label>
                    <Menu.Divider />
                    <Menu.Item component={Link} to="/new" leftSection={<IconPlus size={16} />}>
                      {t('nav.newEvent')}
                    </Menu.Item>
                    <Menu.Item color="red" leftSection={<IconLogout size={16} />} onClick={() => void signOut()}>
                      {t('nav.signOut')}
                    </Menu.Item>
                  </Menu.Dropdown>
                </Menu>
              )}
            </Group>
          </Group>
        </Container>
      </AppShell.Header>
      <AppShell.Main>
        <Container size="lg" py="xl">
          <Outlet />
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
