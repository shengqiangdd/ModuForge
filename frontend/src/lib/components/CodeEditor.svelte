<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EditorView, basicSetup } from 'codemirror';
  import { EditorState } from '@codemirror/state';
  import { oneDark } from '@codemirror/theme-one-dark';
  import { javascript } from '@codemirror/lang-javascript';
  import { python } from '@codemirror/lang-python';
  import { html } from '@codemirror/lang-html';
  import { css } from '@codemirror/lang-css';
  import { json } from '@codemirror/lang-json';
  import { xml } from '@codemirror/lang-xml';
  import { keymap } from '@codemirror/view';
  import { indentWithTab } from '@codemirror/commands';

  let { value = '', language = 'javascript', onChange = () => {} }: {
    value?: string;
    language?: string;
    onChange?: (val: string) => void;
  } = $props();

  let container: HTMLDivElement;
  let view: EditorView;

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

  onMount(() => {
    const state = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        oneDark,
        getLanguageExtension(language),
        keymap.of([indentWithTab]),
        EditorView.updateListener.of(update => {
          if (update.docChanged) {
            onChange(update.state.doc.toString());
          }
        }),
        EditorView.theme({
          '&': { height: '100%' },
          '.cm-scroller': { overflow: 'auto' },
          '.cm-content': { fontFamily: '"Fira Code", "JetBrains Mono", "Consolas", monospace', fontSize: '14px' },
        }),
      ],
    });

    view = new EditorView({ state, parent: container });
  });

  onDestroy(() => {
    view?.destroy();
  });
</script>

<div bind:this={container} class="editor-container h-full w-full overflow-hidden rounded-lg border border-gray-700"></div>
