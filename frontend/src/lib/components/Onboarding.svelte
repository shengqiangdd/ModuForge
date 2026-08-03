<script lang="ts">
  import { t } from '$lib/i18n';

  let { onDone }: { onDone?: () => void } = $props();

  let step = $state(0);

  const steps = [
    {
      icon: 'extension',
      title: $t('onboarding.welcome.title'),
      desc: $t('onboarding.welcome.desc'),
      gradient: 'from-violet-500 to-purple-600',
    },
    {
      icon: 'psychology',
      title: $t('onboarding.ai.title'),
      desc: $t('onboarding.ai.desc'),
      gradient: 'from-cyan-500 to-blue-600',
    },
    {
      icon: 'folder',
      title: $t('onboarding.project.title'),
      desc: $t('onboarding.project.desc'),
      gradient: 'from-emerald-500 to-green-600',
    },
    {
      icon: 'build',
      title: $t('onboarding.build.title'),
      desc: $t('onboarding.build.desc'),
      gradient: 'from-amber-500 to-orange-600',
    },
    {
      icon: 'storefront',
      title: $t('onboarding.publish.title'),
      desc: $t('onboarding.publish.desc'),
      gradient: 'from-pink-500 to-rose-600',
    },
  ];

  function next() {
    if (step < steps.length - 1) {
      step++;
    } else {
      finish();
    }
  }

  function skip() {
    finish();
  }

  function finish() {
    localStorage.setItem('moduforge_onboarded', 'true');
    onDone?.();
  }
</script>

<div class="fixed inset-0 z-[100] flex items-center justify-center p-4 animate-[fadeIn_0.3s_ease-out]"
     style="background: rgba(0,0,0,0.7); backdrop-filter: blur(12px)">
  <div class="w-full max-w-md animate-[scaleIn_0.3s_ease-out]">
    <!-- Step indicator -->
    <div class="flex items-center justify-center gap-2 mb-8">
      {#each steps as _, i}
        <div class="h-1.5 rounded-full transition-all duration-300"
             style="width: {i === step ? '24px' : '8px'}; background: {i <= step ? 'var(--color-primary)' : 'var(--color-border)'}">
        </div>
      {/each}
    </div>

    <!-- Card -->
    <div class="rounded-3xl p-8 text-center border animate-[scaleIn_0.3s_ease-out]"
         style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: 0 32px 64px rgba(0,0,0,0.3)">
      <!-- Icon -->
      <div class="w-20 h-20 rounded-2xl flex items-center justify-center mx-auto mb-6 animate-bounce-subtle"
           style="background: linear-gradient(135deg, {steps[step].gradient})">
        <span class="material-symbols-outlined text-white text-4xl">{steps[step].icon}</span>
      </div>

      <!-- Title -->
      <h2 class="text-2xl font-bold mb-3" style="color: var(--color-text)">{steps[step].title}</h2>

      <!-- Description -->
      <p class="text-sm leading-relaxed mb-8" style="color: var(--color-text-secondary)">{steps[step].desc}</p>

      <!-- Step number -->
      <p class="text-xs mb-6" style="color: var(--color-text-muted)">
        {$t('onboarding.step', { current: step + 1, total: steps.length })}
      </p>

      <!-- Actions -->
      <div class="flex gap-3">
        <button class="flex-1 py-2.5 rounded-xl text-sm font-medium transition-colors"
                style="border: 1px solid var(--color-border); color: var(--color-text-secondary)"
                onclick={skip}>
          {$t('onboarding.skip')}
        </button>
        <button class="flex-1 py-2.5 rounded-xl text-sm font-medium text-white transition-all hover:shadow-lg"
                style="background: var(--gradient-brand)"
                onclick={next}>
          {step < steps.length - 1 ? $t('onboarding.next') : $t('onboarding.done')}
        </button>
      </div>
    </div>
  </div>
</div>
