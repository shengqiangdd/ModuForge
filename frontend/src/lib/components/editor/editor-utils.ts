/**
 * Editor utility functions extracted from EditorWorkspace.
 * Pure functions for file type detection, icon mapping, and language detection.
 */

const _iconCache = new Map<string, string>();
const _iconColorCache = new Map<string, string>();

const languageMap: Record<string, string> = {
  js: 'javascript', jsx: 'javascript', ts: 'javascript', tsx: 'javascript',
  py: 'python', html: 'html', htm: 'html', css: 'css', scss: 'css',
  json: 'json', xml: 'xml', yaml: 'json', yml: 'json', sh: 'shell', bash: 'shell',
};

const iconMap: Record<string, string> = {
  js: 'javascript', jsx: 'javascript', ts: 'javascript', tsx: 'javascript',
  py: 'python', html: 'html', htm: 'html', css: 'css', scss: 'css',
  json: 'data_object', xml: 'code', yaml: 'code', yml: 'code',
  sh: 'terminal', bash: 'terminal',
  md: 'description', txt: 'description', log: 'description',
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', svg: 'image',
  zip: 'folder_zip', tar: 'folder_zip', gz: 'folder_zip',
  prop: 'settings', mk: 'build',
};

const colorMap: Record<string, string> = {
  js: '#f7df1e', jsx: '#61dafb', ts: '#3178c6', tsx: '#61dafb',
  py: '#3776ab', html: '#e34f26', css: '#1572b6',
  json: '#292929', sh: '#4eaa25', bash: '#4eaa25',
  md: '#ffffff', prop: '#8b5cf6',
};

/**
 * Detect the programming language from a file path for syntax highlighting.
 */
export function detectLanguage(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() || '';
  return languageMap[ext] || 'javascript';
}

/**
 * Get the Material Symbols icon name for a file based on its extension.
 */
export function getFileIcon(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() || '';
  if (_iconCache.has(ext)) return _iconCache.get(ext)!;
  const icon = iconMap[ext] || 'description';
  _iconCache.set(ext, icon);
  return icon;
}

/**
 * Get the accent color for a file icon based on its extension.
 */
export function getFileIconColor(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() || '';
  if (_iconColorCache.has(ext)) return _iconColorCache.get(ext)!;
  const color = colorMap[ext] || 'var(--color-text-muted)';
  _iconColorCache.set(ext, color);
  return color;
}

// ─── Tree View ───

export interface TreeNode {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  children?: TreeNode[];
}

/**
 * Build a tree structure from a flat file list.
 */
export function buildTree(fileList: { path: string; size?: number }[]): TreeNode {
  const root: TreeNode = { name: '', path: '', type: 'directory', children: [] };
  for (const file of fileList) {
    const parts = file.path.split('/');
    let current = root;
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isFile = i === parts.length - 1;
      const path = parts.slice(0, i + 1).join('/');
      if (isFile) {
        current.children?.push({ name: part, path, type: 'file', size: file.size });
      } else {
        let dir = current.children?.find(c => c.name === part && c.type === 'directory');
        if (!dir) {
          dir = { name: part, path, type: 'directory', children: [] };
          current.children?.push(dir);
        }
        current = dir;
      }
    }
  }
  sortTreeNodes(root);
  return root;
}

/**
 * Sort tree nodes: directories first, then alphabetically.
 */
export function sortTreeNodes(node: TreeNode) {
  if (!node.children || node.children.length === 0) return;
  node.children.sort((a, b) => {
    if (a.type !== b.type) return a.type === 'directory' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
  for (const child of node.children) {
    if (child.type === 'directory') sortTreeNodes(child);
  }
}

/**
 * Flat view folder-first sort: files inside folders first (grouped by
 * directory), root-level files last, each alphabetical.
 */
export function folderFirstCompare(a: { path: string }, b: { path: string }): number {
  const aIdx = a.path.lastIndexOf('/');
  const bIdx = b.path.lastIndexOf('/');
  const aDir = aIdx === -1 ? '' : a.path.slice(0, aIdx);
  const bDir = bIdx === -1 ? '' : b.path.slice(0, bIdx);
  if (aDir && !bDir) return -1;
  if (!aDir && bDir) return 1;
  if (aDir !== bDir) return aDir.localeCompare(bDir);
  return a.path.localeCompare(b.path);
}

// ─── Security Scan ───

export interface SecurityIssue {
  severity: string;
  file: string;
  line: number;
  rule: string;
  message: string;
}

export interface SecurityScanResult {
  safe: boolean;
  issues: SecurityIssue[];
  score: number;
  summary: string;
}

export function getSecurityIcon(result: SecurityScanResult | null): string {
  if (!result) return 'security';
  return result.safe ? 'verified' : 'warning';
}

export function getSecurityColor(result: SecurityScanResult | null): string {
  if (!result) return 'var(--color-text-muted)';
  return result.safe ? '#22c55e' : '#ef4444';
}

export function getIssueIcon(severity: string): string {
  return severity === 'critical' ? 'error' : severity === 'warning' ? 'warning' : 'info';
}

export function getIssueColor(severity: string): string {
  return severity === 'critical' ? '#ef4444' : severity === 'warning' ? '#f59e0b' : '#6b7280';
}
