<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { basicSetup } from 'codemirror';
  import { EditorView, keymap } from '@codemirror/view';
  import { EditorState, Compartment } from '@codemirror/state';
  import { oneDark } from '@codemirror/theme-one-dark';
  import { javascript } from '@codemirror/lang-javascript';
  import { python } from '@codemirror/lang-python';
  import { html } from '@codemirror/lang-html';
  import { css } from '@codemirror/lang-css';
  import { json } from '@codemirror/lang-json';
  import { xml } from '@codemirror/lang-xml';
  import { indentWithTab } from '@codemirror/commands';
  import { history, historyKeymap, redo, undo, toggleComment } from '@codemirror/commands';

  let {
    value = '',
    language = 'javascript',
    onChange = () => {},
    onSave = () => {},
    onFileSearch = () => {},
    diffMode = false,
    diffContent = '',
    diffLabel = '',
  }: {
    value?: string;
    language?: string;
    onChange?: (val: string) => void;
    onSave?: () => void;
    onFileSearch?: () => void;
    diffMode?: boolean;
    diffContent?: string;
    diffLabel?: string;
  } = $props();

  let container = $state<HTMLDivElement>();
  let view: EditorView;
  let isDirty = $state(false);
  let showSearch = $state(false);
  let searchQuery = $state('');
  let searchResults: {line: number; text: string}[] = $state([]);
  let searchIndex = $state(0);
  let searchInput: HTMLInputElement | undefined = $state();

  const langCompartment = new Compartment();

  function getLanguageExtension(lang: string) {
    switch (lang) {
      case 'python': return python();
      case 'html': return html();
      case 'css': return css();
      case 'json': return json();
      case 'xml': return xml();
      case 'shell': return [];
      case 'elixir': return [];
      default: return javascript();
    }
  }

  function toggleDiff() {
    diffMode = !diffMode;
  }

  function performSearch(query: string) {
    if (!query || !view) return;
    const doc = view.state.doc.toString().toLowerCase();
    const q = query.toLowerCase();
    const results: {line: number; text: string}[] = [];
    const lines = doc.split('\n');
    lines.forEach((line, i) => {
      if (line.includes(q)) {
        results.push({line: i + 1, text: lines[i].trim().slice(0, 80)});
      }
    });
    searchResults = results;
    searchIndex = 0;
    if (results.length > 0) {
      const pos = view.state.doc.line(results[0].line);
      view.dispatch({effects: EditorView.scrollIntoView(pos.from, {y: 'center'})});
    }
  }

  function goToSearchResult(idx: number) {
    if (searchResults.length === 0) return;
    searchIndex = ((idx % searchResults.length) + searchResults.length) % searchResults.length;
    const result = searchResults[searchIndex];
    const pos = view.state.doc.line(result.line);
    view.dispatch({effects: EditorView.scrollIntoView(pos.from, {y: 'center'})});
  }

  function closeSearch() {
    showSearch = false;
    searchQuery = '';
    searchResults = [];
  }

  let lastValue = $state('');

  onMount(() => {
    const state = EditorState.create({
      doc: diffMode && diffContent ? diffContent : value,
      extensions: [
        basicSetup,
        oneDark,
        langCompartment.of(getLanguageExtension(language)),
        history(),
        keymap.of([
          indentWithTab,
          ...historyKeymap,
          {
            key: 'Mod-s',
            run: () => {
              onSave();
              isDirty = false;
              return true;
            },
          },
          {
            key: 'Mod-p',
            run: () => {
              onFileSearch();
              return true;
            },
          },
          {
            key: 'Mod-f',
            run: () => {
              showSearch = true;
              setTimeout(() => searchInput?.focus(), 50);
              return true;
            },
          },
          {
            key: 'Mod-/',
            run: () => { toggleComment(view); return true; },
          },
        ]),
        EditorView.updateListener.of(update => {
          if (update.docChanged && !diffMode) {
            isDirty = true;
            onChange(update.state.doc.toString());
          }
        }),
        EditorView.theme({
          '&': { height: '100%', display: 'flex', flexDirection: 'column' },
          '.cm-scroller': { overflow: 'auto', flex: '1 1 0' },
          '.cm-content': { fontFamily: 'monospace', fontSize: '14px', caretColor: '#fff' },
          '.cm-gutters': { fontFamily: 'monospace', fontSize: '12px' },
          '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { background: '#3a3a5c !important' },
          '.cm-activeLine': { background: 'rgba(255,255,255,0.03)' },
          '.cm-activeLineGutter': { background: 'rgba(255,255,255,0.05)' },
        }),
      ],
    });

    view = new EditorView({ state, parent: container });
    lastValue = value;
  });

  // Update editor content when value prop changes (file switch)
  $effect(() => {
    if (view && value !== lastValue && !diffMode) {
      const currentContent = view.state.doc.toString();
      if (currentContent !== value) {
        view.dispatch({
          changes: { from: 0, to: currentContent.length, insert: value },
        });
      }
      lastValue = value;
    }
  });

  onDestroy(() => {
    view?.destroy();
  });

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (e.shiftKey) goToSearchResult(searchIndex - 1);
      else goToSearchResult(searchIndex + 1);
    }
    if (e.key === 'Escape') closeSearch();
  }
</script>

<div class="relative h-full w-full flex flex-col overflow-hidden rounded-lg border border-gray-700">
  {#if diffMode}
    <div class="flex items-center justify-between px-3 py-1.5 bg-[#1a1a2e] border-b border-gray-700 text-xs text-gray-400 flex-shrink-0">
      <div class="flex items-center gap-2">
        <span class="material-symbols-outlined text-[14px]">compare_arrows</span>
        <span>Diff 视图</span>
        {#if diffLabel}
          <span class="text-gray-500">|</span>
          <span class="text-gray-500">{diffLabel}</span>
        {/if}
      </div>
      <button class="px-2 py-0.5 rounded text-xs hover:bg-gray-700 transition-colors" onclick={() => toggleDiff()}>
        返回编辑
      </button>
    </div>
    <div class="flex-1 flex overflow-hidden">
      <div class="flex-1 overflow-hidden border-r border-gray-700">
        <div class="px-2 py-1 text-xs text-gray-500 bg-[#1e1e2e] border-b border-gray-700">当前版本</div>
        <div bind:this={container} class="h-full w-full overflow-hidden"></div>
      </div>
      <div class="flex-1 overflow-hidden">
        <div class="px-2 py-1 text-xs text-gray-500 bg-[#1e1e2e] border-b border-gray-700">{diffLabel || '对比版本'}</div>
        <pre class="h-full w-full p-4 text-xs font-mono leading-relaxed overflow-auto" style="color: #cdd6f4; background: #1e1e2e;">{diffContent || ''}</pre>
      </div>
    </div>
  {:else}
    <div bind:this={container} class="flex-1 h-full overflow-hidden"></div>
  {/if}

  <!-- File Search Overlay (Ctrl+P) -->
  {#if showSearch}
    <div class="absolute inset-0 z-10 flex items-start justify-center pt-16" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) closeSearch(); }}>
      <div class="w-96 rounded-xl shadow-2xl border border-gray-700 overflow-hidden" style="background: #1e1e2e;" role="presentation" onclick={(e) => e.stopPropagation()}>
        <div class="px-3 py-2 border-b border-gray-700">
          <input
            bind:this={searchInput}
            type="text"
            class="w-full px-3 py-2 rounded-lg text-sm bg-[#181825] border border-gray-600 text-gray-200 outline-none focus:border-gray-500"
            placeholder="搜索文件..."
            bind:value={searchQuery}
            oninput={() => performSearch(searchQuery)}
            onkeydown={handleSearchKeydown}
          />
        </div>
        {#if searchResults.length > 0}
          <div class="max-h-60 overflow-y-auto p-2 space-y-0.5">
            {#each searchResults as result, i}
              <button
                class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs text-left transition-colors
                  {i === searchIndex ? 'bg-gray-700 text-gray-100' : 'text-gray-400 hover:bg-gray-800'}"
                onclick={() => { goToSearchResult(i); closeSearch(); }}
              >
                <span class="text-gray-500 w-8 text-right font-mono">{result.line}</span>
                <span class="truncate flex-1">{result.text}</span>
              </button>
            {/each}
          </div>
          <div class="px-3 py-1.5 border-t border-gray-700 text-[10px] text-gray-500 flex items-center justify-between">
            <span>{searchIndex + 1} / {searchResults.length} 个结果</span>
            <span class="flex items-center gap-2">
              <kbd class="px-1 py-0.5 rounded bg-gray-800 text-gray-400">↑↓</kbd> 导航
              <kbd class="px-1 py-0.5 rounded bg-gray-800 text-gray-400">Esc</kbd> 关闭
            </span>
          </div>
        {:else if searchQuery}
          <div class="px-4 py-6 text-center text-xs text-gray-500">未找到匹配文件</div>
        {/if}
      </div>
    </div>
  {/if}
</div>
