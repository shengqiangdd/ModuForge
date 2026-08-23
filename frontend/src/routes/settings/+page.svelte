<script lang="ts">
  import { onMount } from 'svelte';
  import { getTheme } from '$lib/stores/theme';
  import ProfileSection from './components/ProfileSection.svelte';
  import SecuritySection from './components/SecuritySection.svelte';
  import ProviderSection from './components/ProviderSection.svelte';
  import GitTokenSection from './components/GitTokenSection.svelte';
  import ShortcutsSection from './components/ShortcutsSection.svelte';
  import AppearanceSection from './components/AppearanceSection.svelte';
  import AdvancedSection from './components/AdvancedSection.svelte';
  import DangerZone from './components/DangerZone.svelte';
  import AgentSettingsSection from './components/AgentSettingsSection.svelte';
  import EmailConfigSection from './components/EmailConfigSection.svelte';
  import FavoritesSection from './components/FavoritesSection.svelte';
  import SearchHistorySection from './components/SearchHistorySection.svelte';
  import CustomSkillsSection from './components/CustomSkillsSection.svelte';
  import AboutSection from './components/AboutSection.svelte';
  import PwaInstallSection from './components/PwaInstallSection.svelte';
  import FeatureFlagsSection from './components/FeatureFlagsSection.svelte';

  let themeMode = $state(getTheme());
  let isAdmin = $state(false);

  function onProfileLoaded(data: any) {
    isAdmin = data.isAdmin ?? data;
  }
</script>

<div class="w-full max-w-4xl mx-auto p-4 md:p-6 space-y-8">
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)]">设置</h1>
    <p class="text-sm text-[var(--color-text-secondary)] mt-0.5">管理你的 ModuForge 配置</p>
  </div>

  <ProfileSection onProfileLoaded={onProfileLoaded} />

  {#if isAdmin}
    <EmailConfigSection />
    <FeatureFlagsSection />
  {/if}

  <AgentSettingsSection />
  <ProviderSection />
  <GitTokenSection />
  <ShortcutsSection />
  <AppearanceSection themeMode={themeMode} onThemeChange={(mode) => { themeMode = mode; }} />
  <FavoritesSection />
  <SearchHistorySection />
  <AdvancedSection isAdmin={isAdmin} />
  <DangerZone />
  <SecuritySection />
  <PwaInstallSection />
  <CustomSkillsSection />
  <AboutSection />
</div>
