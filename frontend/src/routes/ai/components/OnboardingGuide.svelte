<script lang="ts">
  import { focusTrap } from '$lib/utils/focusTrap';
  let { show, onClose, onComplete }: { show: boolean; onClose: () => void; onComplete: () => void } = $props();
  let step = $state(0);

  const steps = [
    {
      icon: 'psychology',
      title: '欢迎使用 ModuForge AI',
      desc: '一个强大的 Android 模块开发助手，帮你快速生成、修复和管理模块。',
      tip: '按 ? 随时查看快捷键'
    },
    {
      icon: 'category',
      title: '6 种工作模式',
      desc: '对话（通用问答）、生成（模块生成）、自动构建（一键完成）、需求收集（引导式）、修复（日志分析）、分析（代码审查）',
      tip: '按数字键 1-6 快速切换模式'
    },
    {
      icon: 'folder_open',
      title: '项目上下文',
      desc: '选择一个项目作为上下文，Agent 会更精准地读写文件，避免重复操作。',
      tip: '点击工具栏的项目图标打开'
    }
  ];

  function next() {
    if (step < steps.length - 1) { step++; }
    else { onComplete(); onClose(); }
  }
  function skip() { onComplete(); onClose(); }
</script>

{#if show}
  <div class="onboarding-overlay" role="dialog" aria-label="新手引导" aria-modal="true" use:focusTrap>
    <div class="onboarding-card">
      <div class="step-indicator">
        {#each steps as _, i}
          <span class="dot" class:active={i === step} class:done={i < step}></span>
        {/each}
      </div>

      <div class="step-content">
        <div class="step-icon">
          <span class="material-symbols-outlined">{steps[step].icon}</span>
        </div>
        <h2>{steps[step].title}</h2>
        <p>{steps[step].desc}</p>
        <div class="tip-badge">💡 {steps[step].tip}</div>
      </div>

      <div class="step-actions">
        <button class="btn-skip" onclick={skip}>跳过</button>
        <button class="btn-next" onclick={next}>
          {step < steps.length - 1 ? '下一步' : '开始使用'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .onboarding-overlay {
    position: fixed; inset: 0; z-index: 9999;
    background: rgba(0,0,0,0.6); backdrop-filter: blur(4px);
    display: flex; align-items: center; justify-content: center;
    animation: fadeIn 0.2s ease;
  }
  .onboarding-card {
    background: var(--color-surface, #fff);
    border-radius: 16px; padding: 32px;
    max-width: 420px; width: 90%;
    box-shadow: 0 20px 60px rgba(0,0,0,0.3);
    animation: slideUp 0.3s ease;
  }
  .step-indicator {
    display: flex; gap: 8px; justify-content: center; margin-bottom: 24px;
  }
  .dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--color-border, #ddd);
    transition: all 0.3s ease;
  }
  .dot.active { background: var(--color-primary); width: 24px; border-radius: 4px; }
  .dot.done { background: var(--color-primary); opacity: 0.5; }
  .step-content { text-align: center; }
  .step-icon {
    width: 64px; height: 64px; border-radius: 16px;
    background: var(--gradient-brand-subtle, #f0f0ff);
    display: flex; align-items: center; justify-content: center;
    margin: 0 auto 16px;
  }
  .step-icon .material-symbols-outlined { font-size: 32px; color: var(--color-primary); }
  h2 { font-size: 20px; font-weight: 600; margin: 0 0 8px; color: var(--color-text); }
  p { font-size: 14px; color: var(--color-text-muted); margin: 0 0 16px; line-height: 1.6; }
  .tip-badge {
    display: inline-block; padding: 6px 12px; border-radius: 8px;
    background: var(--color-bg-secondary, #f5f5f5);
    font-size: 12px; color: var(--color-text-muted);
  }
  .step-actions {
    display: flex; gap: 12px; margin-top: 24px;
  }
  .btn-skip {
    flex: 1; padding: 10px; border: 1px solid var(--color-border, #ddd);
    border-radius: 8px; background: transparent; cursor: pointer;
    font-size: 14px; color: var(--color-text-muted);
    transition: all 0.15s;
  }
  .btn-skip:hover { background: var(--color-bg-secondary, #f5f5f5); }
  .btn-next {
    flex: 2; padding: 10px; border: none; border-radius: 8px;
    background: var(--color-primary); color: #fff;
    font-size: 14px; font-weight: 500; cursor: pointer;
    transition: all 0.15s;
  }
  .btn-next:hover { opacity: 0.9; transform: translateY(-1px); }
  @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
  @keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
</style>
