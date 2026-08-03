// ─── Project context management ───
import { fetchProjectFiles, fetchProjectList } from './history';
import { toast } from '$lib/stores/toast.svelte';

/** Load project files into state. Returns files array. */
export async function loadProjectFilesState(projectId: string): Promise<{
  autoBuildFiles: { path: string; content: string; size: number }[];
  generatedFiles: { path: string; content: string }[];
}> {
  const files = await fetchProjectFiles(projectId);
  return {
    autoBuildFiles: files,
    generatedFiles: files.map((f: any) => ({ path: f.path, content: f.content })),
  };
}

/** Load project list for context panel. */
export async function loadContextProjectListState(): Promise<{ id: string; name: string }[]> {
  return fetchProjectList();
}

/** Add a file path to project context string. */
export function addToContextString(context: string, filePath: string): string {
  if (!filePath) return context;
  if (context.includes(filePath)) return context;
  return context ? context + '\n' + filePath : filePath;
}
