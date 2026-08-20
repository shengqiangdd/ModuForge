/**
 * Import dialog state and operations.
 */
import { toast } from '$lib/stores/toast.svelte';
import type { SecurityScanResult } from '../types';
import { memoExtractFiles } from '../markdown';
import {
  loadImportProjects as loadImportProjectsFromBackend,
  scanFiles,
  importFilesToProject,
} from '../import-scan';

export interface ImportState {
  showImportDialog: boolean;
  importFiles: { path: string; content: string }[];
  importProjects: { id: string; name: string }[];
  selectedImportProject: string;
  importing: boolean;
  scanResult: SecurityScanResult | null;
  scanning: boolean;
  showSecurityWarning: boolean;
  pendingImportFiles: { path: string; content: string }[];
}

export function createImportState(): ImportState {
  return {
    showImportDialog: false,
    importFiles: [],
    importProjects: [],
    selectedImportProject: '',
    importing: false,
    scanResult: null,
    scanning: false,
    showSecurityWarning: false,
    pendingImportFiles: [],
  };
}

async function loadImportProjects(s: ImportState): Promise<void> {
  s.importProjects = await loadImportProjectsFromBackend();
  if (s.importProjects.length > 0) s.selectedImportProject = s.importProjects[0].id;
}

export function openImportDialog(s: ImportState, msgContent: string): void {
  const files = memoExtractFiles(msgContent);
  if (!files) return;
  s.importFiles = files;
  s.scanResult = null;
  loadImportProjects(s);
  s.showImportDialog = true;
}

export async function scanAndImport(s: ImportState): Promise<void> {
  if (!s.selectedImportProject || s.importFiles.length === 0) return;
  s.scanning = true;
  s.scanResult = await scanFiles(s.importFiles);
  s.scanning = false;

  if (s.scanResult && !s.scanResult.safe) {
    const criticalIssues = s.scanResult.issues.filter(i => i.severity === 'critical');
    if (criticalIssues.length > 0) {
      s.showSecurityWarning = true;
      s.pendingImportFiles = s.importFiles;
      return;
    }
  }
  proceedImport(s);
}

export function proceedImport(s: ImportState): void {
  s.showSecurityWarning = false;
  doImport(s);
}

export function continueImportAfterWarning(s: ImportState): void {
  s.showSecurityWarning = false;
  doImport(s);
}

async function doImport(s: ImportState): Promise<void> {
  if (!s.selectedImportProject || s.importFiles.length === 0) return;
  s.importing = true;
  const result = await importFilesToProject(s.selectedImportProject, s.importFiles);
  s.importing = false;
  if (result.fail === 0) {
    toast(`成功导入 ${result.success} 个文件到项目`, 'success');
    s.showImportDialog = false;
  } else {
    toast(
      `导入完成：${result.success} 成功，${result.fail} 失败`,
      result.success > 0 ? 'warning' : 'error',
    );
  }
}
