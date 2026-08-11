<script lang="ts">
  let {
    projectId = '',
    selectedTarget = $bindable('arm64'),
    triggerMode = $bindable('manual'),
    building = false,
    status = '',
    project = null,
    buildCached = false,
    savingGitConfig = false,
    savingSchedule = false,
    gitConfig = { url: '', branch: 'main', commitMsg: '', author: '', excludePatterns: '', includePatterns: '', token: '' },
    scheduleConfig = { cron: '0 2 * * *', target: 'universal', arch: 'arm64' },
    buildSchedules = [],
    onStartBuild,
    onCancelBuild,
    onClearCache,
    onSaveGitConfig,
    onSaveSchedule,
    onToggleSchedule,
    onDeleteSchedule,
    onSelectTarget,
    onSelectTriggerMode,
    onGitConfigChange,
    onScheduleConfigChange,
  }: {
    projectId?: string;
    selectedTarget?: string;
    triggerMode?: 'manual' | 'git' | 'schedule';
    building?: boolean;
    status?: string;
    project?: { id: string; name: string; path: string } | null;
    buildCached?: boolean;
    savingGitConfig?: boolean;
    savingSchedule?: boolean;
    gitConfig?: { url: string; branch: string; commitMsg?: string; author?: string; excludePatterns?: string; includePatterns?: string; token?: string };
    scheduleConfig?: { cron: string; target: string; arch: string };
    buildSchedules?: { id: string; cron: string; target: string; arch: string; active: boolean }[];
    onStartBuild?: () => void;
    onCancelBuild?: () => void;
    onClearCache?: () => void;
    onSaveGitConfig?: () => void;
    onSaveSchedule?: () => void;
    onToggleSchedule?: (id: string, active: boolean) => void;
    onDeleteSchedule?: (id: string) => void;
    onSelectTarget?: (t: string) => void;
    onSelectTriggerMode?: (m: 'manual' | 'git' | 'schedule') => void;
    onGitConfigChange?: (cfg: { url: string; branch: string; commitMsg?: string; author?: string; excludePatterns?: string; includePatterns?: string; token?: string }) => void;
    onScheduleConfigChange?: (cfg: { cron: string; target: string; arch: string }) => void;
  } = $props();

  const targets = [
    { value: 'arm64', label: 'arm64', icon: 'smartphone', desc: '64-bit ARM (现代设备)' },
    { value: 'arm', label: 'arm', icon: 'phone_android', desc: '32-bit ARM (旧设备)' },
    { value: 'x86_64', label: 'x86_64', icon: 'computer', desc: 'x86_64 (模拟器)' },
  ];
  const triggerModes: ('manual' | 'git' | 'schedule')[] = ['manual', 'git', 'schedule'];
</script>

<!-- Architecture Selection -->
<div class="mb-6">
  <span class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">目标架构</span>
  <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
    {#each targets as t}
      <button
        class="relative p-4 rounded-2xl border-2 transition-all duration-200 text-left cursor-pointer
          {selectedTarget === t.value
            ? 'border-primary-500 shadow-glow'
            : 'border-[var(--color-border)] hover:border-[var(--color-text-muted)] hover:bg-[var(--color-surface)]'}"
        style={selectedTarget === t.value ? 'background: color-mix(in srgb, var(--color-primary) 15%, transparent)' : ''}
        onclick={() => onSelectTarget?.(t.value)}
        disabled={building}
      >
        {#if selectedTarget === t.value}
          <div class="absolute top-2 right-2 w-5 h-5 rounded-full flex items-center justify-center" style="background: var(--color-primary)">
            <span class="material-symbols-outlined text-white text-[12px]">check</span>
          </div>
        {/if}
        <div class="flex items-center gap-3 mb-2">
          <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center">
            <span class="material-symbols-outlined text-white text-lg">{t.icon}</span>
          </div>
          <span class="text-base font-semibold text-[var(--color-text)]">{t.label}</span>
        </div>
        <p class="text-xs text-[var(--color-text-muted)]">{t.desc}</p>
      </button>
    {/each}
  </div>
</div>

<!-- Trigger Mode -->
<div class="mb-6">
  <span class="text-sm font-medium text-[var(--color-text-secondary)] mb-3 block">触发方式</span>
  <div class="grid grid-cols-3 gap-2">
    {#each triggerModes as mode (mode)}
      {@const icons = { manual: 'build', git: 'cloud_upload', schedule: 'schedule' }}
      <button
        class="py-2.5 px-2 rounded-xl text-sm font-medium transition-all duration-200 flex items-center justify-center gap-1.5 cursor-pointer"
        style={triggerMode === mode
          ? 'background: var(--color-primary); color: white'
          : 'border: 1px solid var(--color-border); color: var(--color-text-secondary); background: var(--color-bg-elevated)'}
        onclick={() => onSelectTriggerMode?.(mode)}
        disabled={building}
      >
        <span class="material-symbols-outlined text-[18px]">{icons[mode]}</span>
        <span>{mode === 'manual' ? '手动' : mode === 'git' ? '推送' : '定时'}</span>
      </button>
    {/each}
  </div>
</div>

<!-- Git Push Config Panel -->
{#if triggerMode === 'git'}
  <div class="mb-6 p-4 rounded-2xl border" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
    <div class="flex items-center gap-2 mb-3">
      <span class="material-symbols-outlined text-[18px] text-[var(--color-info)]">cloud_upload</span>
      <span class="text-sm font-semibold text-[var(--color-text)]">推送触发配置</span>
    </div>
    <p class="text-xs text-[var(--color-text-muted)] mb-3">配置 Git 仓库地址、认证 Token 和分支，支持推送到 GitHub/GitLab/Bitbucket。</p>
    <div class="space-y-3">
      <div>
        <label for="git-url" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">仓库地址</label>
        <input id="git-url" type="text" class="input-field w-full text-sm" placeholder="https://github.com/user/repo.git" value={gitConfig.url} oninput={(e) => onGitConfigChange?.({ ...gitConfig, url: (e.target as HTMLInputElement).value })} />
      </div>
      <div>
        <label for="git-token" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">认证 Token <span class="text-[var(--color-text-muted)]">(可选，私有仓库必填)</span></label>
        <input id="git-token" type="password" class="input-field w-full text-sm" placeholder="ghp_xxx / glpat-xxx" value={gitConfig.token || ''} oninput={(e) => onGitConfigChange?.({ ...gitConfig, token: (e.target as HTMLInputElement).value })} />
        <p class="text-[10px] text-[var(--color-text-muted)] mt-1">GitHub: Personal Access Token | GitLab: Access Token | Bitbucket: App Password</p>
      </div>
      <div>
        <label for="git-branch" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">推送分支</label>
        <input id="git-branch" type="text" class="input-field w-full text-sm" placeholder="main" value={gitConfig.branch} oninput={(e) => onGitConfigChange?.({ ...gitConfig, branch: (e.target as HTMLInputElement).value })} />
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label for="git-author" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">提交者</label>
          <input id="git-author" type="text" class="input-field w-full text-sm" placeholder="User Name" value={gitConfig.author || ''} oninput={(e) => onGitConfigChange?.({ ...gitConfig, author: (e.target as HTMLInputElement).value })} />
        </div>
        <div>
          <label for="git-commit-msg" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">提交信息</label>
          <input id="git-commit-msg" type="text" class="input-field w-full text-sm" placeholder="Auto build from push" value={gitConfig.commitMsg || ''} oninput={(e) => onGitConfigChange?.({ ...gitConfig, commitMsg: (e.target as HTMLInputElement).value })} />
        </div>
      </div>
      <!-- File Filtering -->
      <div class="pt-2 border-t" style="border-color: var(--color-border)">
        <div class="flex items-center gap-2 mb-2">
          <span class="material-symbols-outlined text-[14px] text-[var(--color-info)]">filter_alt</span>
          <span class="text-xs font-medium text-[var(--color-text-secondary)]">文件过滤</span>
          <span class="text-[10px] text-[var(--color-text-muted)]">(可选)</span>
        </div>
        <div class="space-y-2">
          <div>
            <label for="git-exclude" class="text-[10px] font-medium text-[var(--color-text-muted)] mb-0.5 block">排除模式 (每行一个)</label>
            <textarea
              id="git-exclude"
              class="input-field w-full text-xs font-mono"
              rows="3"
              placeholder={"*.log\nnode_modules/\nbuild/\n.env"}
              value={gitConfig.excludePatterns || ''}
              oninput={(e) => onGitConfigChange?.({ ...gitConfig, excludePatterns: (e.target as HTMLTextAreaElement).value })}
            ></textarea>
          </div>
          <div>
            <label for="git-include" class="text-[10px] font-medium text-[var(--color-text-muted)] mb-0.5 block">包含模式 (留空=全部，每行一个)</label>
            <input
              id="git-include"
              type="text"
              class="input-field w-full text-xs font-mono"
              placeholder={"src/**\n*.go"}
              value={gitConfig.includePatterns || ''}
              oninput={(e) => onGitConfigChange?.({ ...gitConfig, includePatterns: (e.target as HTMLInputElement).value })}
            />
          </div>
          <p class="text-[10px] text-[var(--color-text-muted)]">默认排除: *.log, node_modules/, build/, dist/, *.zip, .env, .git/, *.exe 等</p>
        </div>
      </div>

      <div class="flex items-center justify-between pt-2">
        <span class="text-xs text-[var(--color-text-muted)]">推送含源码，构建产物发布到 Release</span>
        <button class="px-3 py-1.5 rounded-xl text-xs font-semibold text-white transition-colors" style="background: var(--gradient-brand)" onclick={onSaveGitConfig} disabled={savingGitConfig || !gitConfig.url}>
          {savingGitConfig ? '保存中...' : '保存配置'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Schedule Config Panel -->
{#if triggerMode === 'schedule'}
  <div class="mb-6 p-4 rounded-2xl border" style="border-color: var(--color-border); background: var(--color-bg-elevated)">
    <div class="flex items-center gap-2 mb-3">
      <span class="material-symbols-outlined text-[18px] text-[var(--color-info)]">schedule</span>
      <span class="text-sm font-semibold text-[var(--color-text)]">定时构建配置</span>
    </div>
    <div class="space-y-3">
      <div>
        <label for="schedule-cron" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">Cron 表达式</label>
        <input id="schedule-cron" type="text" class="input-field w-full text-sm" placeholder="0 2 * * *" value={scheduleConfig.cron} oninput={(e) => onScheduleConfigChange?.({ ...scheduleConfig, cron: (e.target as HTMLInputElement).value })} />
        <p class="text-[10px] text-[var(--color-text-muted)] mt-1">分 时 日 月 周 | 例: 每天凌晨2点 → 0 2 * * *</p>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="schedule-target" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">目标平台</label>
          <select id="schedule-target" class="input-field w-full text-sm" value={scheduleConfig.target} onchange={(e) => onScheduleConfigChange?.({ ...scheduleConfig, target: (e.target as HTMLSelectElement).value })}>
            <option value="universal">通用</option>
          </select>
        </div>
        <div>
          <label for="schedule-arch" class="text-xs font-medium text-[var(--color-text-secondary)] mb-1 block">架构</label>
          <select id="schedule-arch" class="input-field w-full text-sm" value={scheduleConfig.arch} onchange={(e) => onScheduleConfigChange?.({ ...scheduleConfig, arch: (e.target as HTMLSelectElement).value })}>
            <option value="arm64">arm64</option>
            <option value="arm">arm</option>
            <option value="x86_64">x86_64</option>
          </select>
        </div>
      </div>
      <button class="px-4 py-2 rounded-xl text-xs font-semibold text-white transition-colors w-full" style="background: var(--gradient-brand)" onclick={onSaveSchedule} disabled={savingSchedule || !scheduleConfig.cron}>
        {savingSchedule ? '创建中...' : '添加定时任务'}
      </button>
    </div>

    {#if buildSchedules.length > 0}
      <div class="mt-3 space-y-2">
        {#each buildSchedules as s}
          <div class="flex items-center justify-between p-2 rounded-lg" style="background: var(--color-surface)">
            <div class="flex items-center gap-2 text-xs">
              <span class="material-symbols-outlined text-[14px] {s.active ? 'text-[var(--color-success)]' : 'text-[var(--color-text-muted)]'}">
                {s.active ? 'check_circle' : 'pause_circle'}
              </span>
              <code class="text-[var(--color-text)]">{s.cron}</code>
              <span class="text-[var(--color-text-muted)]">{s.target}/{s.arch}</span>
            </div>
            <div class="flex items-center gap-1">
              <button class="p-1 rounded hover:bg-[var(--color-border)]" onclick={() => onToggleSchedule?.(s.id, !s.active)} title={s.active ? '暂停' : '启用'}>
                <span class="material-symbols-outlined text-[14px]">{s.active ? 'pause' : 'play_arrow'}</span>
              </button>
              <button class="p-1 rounded hover:bg-[var(--color-error-light)]" onclick={() => onDeleteSchedule?.(s.id)} title="删除">
                <span class="material-symbols-outlined text-[14px] text-[var(--color-error)]">delete</span>
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<!-- Build Button -->
<div class="flex gap-3 mb-6">
  <button
    class="flex-1 py-3 rounded-xl font-semibold text-sm text-white transition-all duration-200 disabled:opacity-50
      bg-gradient-to-r from-primary-600 to-primary-700 hover:from-primary-700 hover:to-primary-800 active:scale-[0.98] shadow-sm hover:shadow-glow flex items-center justify-center gap-2"
    onclick={onStartBuild}
    disabled={building}
  >
    {#if building}
      <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/></svg>
      构建中...
    {:else}
      <span class="material-symbols-outlined text-[18px]">build</span>
      开始构建
    {/if}
  </button>
  {#if building}
    <button class="px-5 py-3 rounded-xl text-sm font-medium transition-colors" style="border: 1px solid var(--color-error); color: var(--color-error); background: transparent" onclick={onCancelBuild}>
      取消
    </button>
  {:else}
    <button class="px-4 py-3 rounded-xl text-sm font-medium transition-colors flex items-center gap-1.5" style="border: 1px solid var(--color-border); color: var(--color-text-secondary); background: transparent" onclick={onClearCache} title="清除构建缓存">
      <span class="material-symbols-outlined text-[16px]">cleaning_services</span>
      清除缓存
    </button>
  {/if}
</div>
