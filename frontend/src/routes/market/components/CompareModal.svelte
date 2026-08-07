<script lang="ts">
  let { show, result, onClose, fmt, compareWinner }: {
    show: boolean;
    result: {
      error?: string;
      title_a?: string;
      title_b?: string;
      description_a?: string;
      description_b?: string;
      version_a?: string;
      version_b?: string;
      category_a?: string;
      category_b?: string;
      author_a?: string;
      author_b?: string;
      license_a?: string;
      license_b?: string;
      rating_a?: number;
      rating_b?: number;
      stars_a?: number;
      stars_b?: number;
      installs_a?: number;
      installs_b?: number;
      dep_count_a?: number;
      dep_count_b?: number;
    } | null;
    onClose: () => void;
    fmt: (n: number) => string;
    compareWinner: (a: number, b: number) => 'a' | 'b' | 'tie';
  } = $props();
</script>

{#if show && result}
  <div class="fixed inset-0 flex items-center justify-center z-50 p-4 animate-[fadeIn_0.15s_ease-out]" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px)" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class="rounded-2xl max-w-3xl w-full max-h-[85vh] overflow-auto border animate-[scaleIn_0.2s_ease-out]" style="background: var(--color-bg-elevated); border-color: var(--color-border); box-shadow: var(--shadow-xl)" role="dialog" aria-modal="true" tabindex="-1">
      <div class="p-5 border-b flex items-center justify-between" style="border-color: var(--color-border)">
        <h3 class="text-lg font-bold text-[var(--color-text)]">模块对比</h3>
        <button class="p-2 rounded-xl hover:bg-[var(--color-surface)] transition-colors" onclick={onClose}>
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>
      {#if result.error}
        <div class="p-8 text-center text-[var(--color-error)]">{result.error}</div>
      {:else}
        <div class="p-5">
          <table class="compare-table">
            <thead>
              <tr>
                <th class="compare-label">字段</th>
                <th class="compare-value" style="color: var(--color-primary)">{result.title_a}</th>
                <th class="compare-value" style="color: var(--color-primary)">{result.title_b}</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td class="compare-label">描述</td>
                <td class="compare-value">{result.description_a || '-'}</td>
                <td class="compare-value">{result.description_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">版本</td>
                <td class="compare-value">{result.version_a || '-'}</td>
                <td class="compare-value">{result.version_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">类别</td>
                <td class="compare-value">{result.category_a || '-'}</td>
                <td class="compare-value">{result.category_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">作者</td>
                <td class="compare-value">{result.author_a || '-'}</td>
                <td class="compare-value">{result.author_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">许可证</td>
                <td class="compare-value">{result.license_a || '-'}</td>
                <td class="compare-value">{result.license_b || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">评分</td>
                <td class="compare-value" class:winner={compareWinner(result.rating_a ?? 0, result.rating_b ?? 0) === 'a'}>{result.rating_a?.toFixed(1) || '-'}</td>
                <td class="compare-value" class:winner={compareWinner(result.rating_a ?? 0, result.rating_b ?? 0) === 'b'}>{result.rating_b?.toFixed(1) || '-'}</td>
              </tr>
              <tr>
                <td class="compare-label">Stars</td>
                <td class="compare-value" class:winner={compareWinner(result.stars_a ?? 0, result.stars_b ?? 0) === 'a'}>{fmt(result.stars_a ?? 0)}</td>
                <td class="compare-value" class:winner={compareWinner(result.stars_a ?? 0, result.stars_b ?? 0) === 'b'}>{fmt(result.stars_b ?? 0)}</td>
              </tr>
              <tr>
                <td class="compare-label">安装量</td>
                <td class="compare-value" class:winner={compareWinner(result.installs_a ?? 0, result.installs_b ?? 0) === 'a'}>{fmt(result.installs_a ?? 0)}</td>
                <td class="compare-value" class:winner={compareWinner(result.installs_a ?? 0, result.installs_b ?? 0) === 'b'}>{fmt(result.installs_b ?? 0)}</td>
              </tr>
              <tr>
                <td class="compare-label">依赖数</td>
                <td class="compare-value" class:winner={compareWinner(result.dep_count_b ?? 0, result.dep_count_a ?? 0) === 'a'}>{result.dep_count_a}</td>
                <td class="compare-value" class:winner={compareWinner(result.dep_count_b ?? 0, result.dep_count_a ?? 0) === 'b'}>{result.dep_count_b}</td>
              </tr>
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .compare-table {
    width: 100%;
    border-collapse: collapse;
  }
  .compare-table th,
  .compare-table td {
    padding: 10px 12px;
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }
  .compare-table th {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--color-text-muted);
  }
  .compare-label {
    width: 80px;
    font-size: 12px;
    font-weight: 500;
    color: var(--color-text-secondary);
  }
  .compare-value {
    font-size: 13px;
    color: var(--color-text);
  }
  .compare-value.winner {
    color: #22c55e;
    font-weight: 600;
  }
  .compare-value.winner::after {
    content: ' ✓';
    font-size: 11px;
  }
</style>