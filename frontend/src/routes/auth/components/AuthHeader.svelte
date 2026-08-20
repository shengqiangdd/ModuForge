<script lang="ts">
  let { isLogin = true }: { isLogin?: boolean } = $props();
</script>

<div class="text-center mb-8">
  <div class="auth-logo w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-5 relative">
    <div class="absolute inset-0 rounded-2xl" style="background: var(--gradient-brand); animation: breatheGlow 3s ease-in-out infinite"></div>
    <span class="material-symbols-outlined text-white text-3xl relative z-10">extension</span>
  </div>
  <h1 class="text-2xl font-bold tracking-tight" style="color: var(--color-text)">ModuForge</h1>
  <p class="text-sm mt-1.5" style="color: var(--color-text-secondary)">{isLogin ? 'Welcome back' : 'Create your account'}</p>
</div>

<style>
  @keyframes breatheGlow {
    0%, 100% { box-shadow: 0 0 20px color-mix(in srgb, var(--color-primary) 30%, transparent), 0 0 40px color-mix(in srgb, var(--color-primary) 10%, transparent); }
    50% { box-shadow: 0 0 30px color-mix(in srgb, var(--color-primary) 50%, transparent), 0 0 60px color-mix(in srgb, var(--color-primary) 20%, transparent), 0 0 80px color-mix(in srgb, var(--color-info) 10%, transparent); }
  }
</style>
