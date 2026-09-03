<script lang="ts">
  import { shortcutLabel, type Shortcut } from '$lib/stores/shortcuts';

  let {
    project = null,
    projectId = '',
    selectedFile = null,
    saving = false,
    formatting = false,
    securityScanning = false,
    securityResult = null,
    showTerminal = false,
    showDiffList = false,
    diffFiles = [],
    shortcuts = [],
    onFormatCode,
    onRunSecurityScan,
    onValidateProject,
    onToggleTerminal,
    onSave,
    getSecurityIcon = () => 'security',
    getSecurityColor = () => 'var(--color-text-muted)',
  }: {
    project?: { id: string; name: string } | null;
    projectId?: string;
    selectedFile?: string | null;
    saving?: boolean;
    formatting?: boolean;
    securityScanning?: boolean;
    securityResult?: { safe: boolean; issues: { severity: string; file: string; line: number; rule: string; message: string }[] } | null;
    showTerminal?: boolean;
    showDiffList?: boolean;
    diffFiles?: { path: string; current: string; incoming: string }[];
    shortcuts?: Shortcut[];
    onFormatCode?: () => void;
    onRunSecurityScan?: () => void;
    onValidateProject?: () => void;
    onToggleTerminal?: () => void;
    onSave?: () => void;
    getSecurityIcon?: () => string;
    getSecurityColor?: () => string;
  } = $props();
</script>

<div class="h-12 flex items-center justify-between px-2 sm:px-4 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex-shrink-0">
  <div class="flex items-center gap-2 sm:gap-3 min-w-0">
    <span class="text-sm font-medium text-[var(--color-text)] hidden sm:block truncate">{project?.name}</span>
    <span class="px-2 py-0.5 rounded-md text-[10px] font-medium" style="background: var(--color-primary-light); color: var(--color-primary)">Universal</span>
    {#if showDiffList && diffFiles.length > 0}
      <span class="text-xs text-primary-500 flex items-center gap-1 hidden sm:flex">
        <span class="material-symbols-outlined text-[14px]">compare_arrows</span>
        {diffFiles.length} 待审查
      </span>
    {/if}
  </div>
  <div class="flex items-center gap-1 sm:gap-2 flex-shrink-0 overflow-x-auto">
    {#if showDiffList && diffFiles.length > 0}
      <button class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium bg-primary-600 text-white hover:bg-primary-700 transition-colors whitespace-nowrap" onclick={onFormatCode}>
        <span class="material-symbols-outlined text-[14px]">check</span>
        <span class="hidden sm:inline">完成审查</span>
      </button>
    {/if}
    <button
      class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap"
      style="background: var(--color-surface); color: var(--color-text-secondary)"
      onclick={onFormatCode}
      disabled={formatting}
      title="格式化代码 ({shortcutLabel(shortcuts.find((s: any) => s.id === 'format-code')!)})"
    >
      <span class="material-symbols-outlined !text-[14px] {formatting ? 'animate-spin' : ''}">{formatting ? 'progress_activity' : 'align_horizontal_left'}</span>
      <span class="hidden md:inline">{formatting ? '格式化中...' : '格式化'}</span>
    </button>
    <button
      class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 whitespace-nowrap"
      style="background: var(--color-surface); color: {getSecurityColor()}"
      onclick={onRunSecurityScan}
      disabled={securityScanning}
      title="安全扫描"
    >
      <span class="material-symbols-outlined !text-[14px] {securityScanning ? 'animate-spin' : ''}">{securityScanning ? 'progress_activity' : getSecurityIcon()}</span>
      <span class="hidden md:inline">{securityScanning ? '扫描中...' : '安全扫描'}</span>
    </button>
    <button class="flex items-center justify-center w-8 h-8 rounded-lg bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors" onclick={onValidateProject} title="项目完整性校验">
      <span class="material-symbols-outlined !text-[16px]">checklist</span>
    </button>
    <a href="/projects/{projectId}/build" class="flex items-center justify-center w-8 h-8 rounded-lg bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors no-underline" title="构建模块">
      <span class="material-symbols-outlined !text-[16px]">build</span>
    </a>
    <a href="/projects/{projectId}/build?target=android" class="flex items-center gap-1 px-2 py-1.5 rounded-lg text-xs font-medium bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] transition-colors no-underline whitespace-nowrap" title="构建 APK">
      <span class="material-symbols-outlined !text-[14px]">android</span>
      <span class="hidden lg:inline">APK</span>
    </a>
    <button
      class="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
      style={showTerminal ? 'background: var(--color-primary); color: white' : 'background: var(--color-surface); color: var(--color-text-secondary)'}
      onclick={onToggleTerminal}
      title="终端 ({shortcutLabel(shortcuts.find((s: any) => s.id === 'toggle-terminal')!)})"
    >
      <span class="material-symbols-outlined !text-[16px]">terminal</span>
    </button>
    <button
      class="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 whitespace-nowrap
        {saving ? 'text-[var(--color-text-muted)]' : 'bg-primary-600 text-white hover:bg-primary-700'}"
      style={saving ? 'background: var(--color-surface)' : ''}
      onclick={onSave}
      disabled={saving || !selectedFile}
    >
      <span class="material-symbols-outlined !text-[14px]">{saving ? 'hourglass_top' : 'save'}</span>
      <span class="hidden sm:inline">{saving ? '保存中...' : '保存'}</span>
    </button>
  </div>
</div>
