import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown', () => {
  it('renders plain text unchanged', () => {
    expect(renderMarkdown('hello world')).toBe('hello world');
  });

  it('wraps h3 with strong + text-sm', () => {
    const result = renderMarkdown('### subtitle');
    expect(result).toContain('<strong class="text-sm block mt-2 mb-1">subtitle</strong>');
  });

  it('wraps h2 with strong + text-base', () => {
    const result = renderMarkdown('## heading');
    expect(result).toContain('<strong class="text-base block mt-3 mb-1">heading</strong>');
  });

  it('wraps h1 with strong + text-lg', () => {
    const result = renderMarkdown('# title');
    expect(result).toContain('<strong class="text-lg block mt-3 mb-1">title</strong>');
  });

  it('renders bold text', () => {
    expect(renderMarkdown('**bold**')).toContain('<strong>bold</strong>');
  });

  it('renders links', () => {
    const result = renderMarkdown('[text](https://example.com)');
    expect(result).toContain('<a href="https://example.com"');
    expect(result).toContain('>text</a>');
  });

  it('converts newlines to <br>', () => {
    expect(renderMarkdown('line1\nline2')).toBe('line1<br>line2');
  });

  it('escapes HTML to prevent XSS', () => {
    const result = renderMarkdown('<script>alert("xss")</script>');
    expect(result).not.toContain('<script>');
    expect(result).toContain('&lt;script&gt;');
  });

  it('handles complex markdown', () => {
    const input = '# Title\n## Subtitle\n**bold** and [link](http://example.com)';
    const result = renderMarkdown(input);
    expect(result).toContain('<strong class="text-lg block mt-3 mb-1">Title</strong>');
    expect(result).toContain('<strong class="text-base block mt-3 mb-1">Subtitle</strong>');
    expect(result).toContain('<strong>bold</strong>');
    expect(result).toContain('<a href="http://example.com"');
  });
});
  it('escapes quotes in links to prevent attribute injection', () => {
    const result = renderMarkdown('[x](" onmouseover="alert(1))');
    // Dangerous URL is stripped entirely (link becomes plain text).
    expect(result).not.toContain('onmouseover=');
    expect(result).not.toContain('href=');
  });

  it('strips javascript: links', () => {
    const result = renderMarkdown('[click](javascript:alert(1))');
    expect(result).not.toContain('href=');
    expect(result).toContain('click');
  });

  it('strips data: links', () => {
    const result = renderMarkdown('[img](data:text/html;base64,PHNjcmlwdD4=)');
    expect(result).not.toContain('href=');
  });

  it('allows http, https, mailto and relative links', () => {
    expect(renderMarkdown('[a](https://x.com)')).toContain('href="https://x.com"');
    expect(renderMarkdown('[a](http://x.com)')).toContain('href="http://x.com"');
    expect(renderMarkdown('[a](mailto:a@b.com)')).toContain('href="mailto:a@b.com"');
    expect(renderMarkdown('[a](/local/path)')).toContain('href="/local/path"');
  });

  it('escapes quotes in raw HTML input', () => {
    const result = renderMarkdown('<img src=x onerror=alert(1)>');
    // Fully escaped: the tag is rendered as plain text, never as an element.
    expect(result).not.toContain('<img');
    expect(result).toContain('&lt;img');
  });
