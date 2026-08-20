export interface MarketModule {
  id: string; title: string; slug: string; description: string; category: string;
  tags: string; version: string; version_code: number; author: string;
  license: string; stars: number; installs: number; updated_at: string; created_at: string;
  screenshots?: { url: string }[];
  cover_image?: string;
  dependencies?: { id: string; min_version?: string; optional?: boolean }[];
}

export interface Review {
  id: string; module_id: string; uid: string; username: string;
  rating: number; comment: string; created_at: string;
}

export interface ModuleVersion {
  id: string; module_id: string; version: string; version_code: string;
  changelog: string; created_at: string;
}

export interface HealthScore {
  score: number; level: string;
  details: { name: string; label: string; score: number; max: number }[];
}

export interface ModuleTag {
  id: number; name: string; color: string;
}

export interface ChangelogEntry {
  id: number; version: string; content: string; created_at: string;
}

export interface InstallStat {
  period: string; count: number;
}

export interface TemplateItem {
  id: number; name: string; description: string; category: string;
  author: string; downloads: number; rating: number; module_data: string; created_at: string;
}

export interface TemplateCategory {
  name: string; count: number;
}

export const categoryStyles: Record<string, string> = {
  system: 'background: rgba(59,130,246,0.15); color: #60a5fa',
  ui: 'background: rgba(168,85,247,0.15); color: #c084fc',
  audio: 'background: rgba(34,197,94,0.15); color: #4ade80',
  display: 'background: rgba(249,115,22,0.15); color: #fb923c',
  utility: 'background: rgba(161,161,170,0.15); color: #a1a1aa',
};

export function getToken(): string {
  return localStorage.getItem('moduforge_token') || '';
}

export function fmt(n: number): string {
  return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n);
}
