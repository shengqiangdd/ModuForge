<script lang="ts">
	import { onMount } from 'svelte';

	const projectId = window.location.pathname.split('/').filter(Boolean).at(-2) || '';
	let versions = $state<any[]>([]);
	let loading = $state(true);
	let selectedVersionA = $state('');
	let selectedVersionB = $state('');
	let comparing = $state(false);
	let diffData = $state<any>(null);
	let selectedFile = $state<string | null>(null);
	let rollbackLoading = $state(false);
	let activeTab = $state<'list'|'diff'|'rollback'>('list');
	let showRollbackConfirm = $state(false);
	let rollbackTarget = $state('');
	let expandingFile = $state<string | null>(null);
	let generatingLineDiff = $state(false);
	let compareWithThis: any = null;
	let expandedId: string | null = null;
	let rollbackStatusMsg = $state('');

	// Parse semver version string
	function parseSemver(version: string): { major: number; minor: number; patch: number; preRelease?: string; releaseType: string } {
		const match = version?.match(/v?(\d+)\.(\d+)\.(\d+)(?:-([a-z0-9\-.]+))?/i);
		if (!match) return { major: 0, minor: 0, patch: 0, releaseType: 'unknown' };
		const [_, maj, min, pat, pre] = match;
		let releaseType = 'release' as const;
		if (pre) {
			if (pre.toLowerCase().includes('alpha')) releaseType = 'alpha';
			else if (pre.toLowerCase().includes('beta')) releaseType = 'beta';
			else if (pre.startsWith('rc')) releaseType = 'candidate';
			else if (pre.includes('+')) releaseType = 'tag';
			else releaseType = 'prerelease';
		}
		return { major: parseInt(maj, 10), minor: parseInt(min, 10), patch: parseInt(pat, 10), preRelease: pre, releaseType };
	}

	function getVersionTag(version: string): { badge: string; color: string } {
		const v = parseSemver(version);
		const tags: Record<string, { badge: string; color: string }> = {
			alpha: { badge: 'Alpha', color: '#ef4444' },
			beta: { badge: 'Beta', color: '#f59e0b' },
			candidate: { badge: 'Candidate', color: '#3b82f6' },
			tag: { badge: 'Tagged', color: '#8b5cf6' },
		};
		return tags[v.releaseType] ?? { badge: 'Release', color: '#22c55e' };
	}

	function timeAgo(dateStr: string): string {
		try {
			const d = new Date(dateStr);
			const s = Math.floor((Date.now() - d.getTime()) / 1000);
			if (s < 60) return '刚刚';
			if (s < 3600) return `${Math.floor(s/60)} 分钟前`;
			if (s < 86400) return `${Math.floor(s/3600)} 小时前`;
			if (s < 2592000) return `${Math.floor(s/86400)} 天前`;
			return d.toLocaleDateString();
		} catch { return dateStr; }
	}

	function formatSize(bytes: number): string {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
		return (bytes / 1048576).toFixed(1) + ' MB';
	}

	async function loadVersions() {
		loading = true;
		try {
			const token = localStorage.getItem('moduforge_token') || '';
			const res = await fetch(`/api/v1/projects/${projectId}/versions`, { headers: { Authorization: `Bearer ${token}` } });
			if (res.ok) {
				const data = await res.json();
				versions = Array.isArray(data) ? data : (data.versions || []);
			}
		} catch (e) { console.error('Load versions failed:', e); }
		loading = false;
	}

	onMount(() => { loadVersions(); });

	async function initiateRollback(version: string) {
		rollbackTarget = version;
		showRollbackConfirm = true;
	}

	async function performRollback(targetVersion: string) {
		showRollbackConfirm = false;
		rollbackLoading = true;
		rollbackStatusMsg = '';
		try {
			const token = localStorage.getItem('moduforge_token') || '';
			const url = `/api/v1/projects/${projectId}/versions/${targetVersion}/rollback`;
			const res = await fetch(url, { method: 'POST', headers: { Authorization: `Bearer ${token}` } });
			if (res.ok) {
				rollbackStatusMsg = `成功回滚到版本 ${targetVersion}`;
				await loadVersions();
				activeTab = 'list';
			} else {
				throw new Error(`回滚失败: ${res.status} ${await res.text()}`);
			}
		} catch (e: any) {
			console.error('Rollback error:', e);
			alert(e.message);
		} finally {
			rollbackLoading = false;
		}
	}

	async function generateDiff() {
		if (!selectedVersionA || !selectedVersionB) { alert('请选择两个版本进行对比'); return; }
		comparing = true;
		diffData = null;
		selectedFile = null;
		generatingLineDiff = false;

		const token = localStorage.getItem('moduforge_token') || '';
		let url = `/api/v1/projects/${projectId}/versions/diff?from=${encodeURIComponent(selectedVersionA)}&to=${encodeURIComponent(selectedVersionB)}`;
		try {
			let res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
			if (!res.ok) {
				url = `/api/v1/projects/${projectId}/compare`;
				res = await fetch(url, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
					body: JSON.stringify({ version_a: selectedVersionA, version_b: selectedVersionB }),
				});
			}
			if (res.ok) {
				diffData = await res.json();
				if (diffData && diffData.diffItems?.length > 0) { selectedFile = diffData.diffItems[0].path; }
			} else { alert('生成差异失败，请检查版本是否存在'); }
		} catch (e: any) { alert('生成差异失败: ' + e.message); }
		finally { comparing = false; }
	}

	async function selectDiffFile(filePath: string) {
		selectedFile = filePath;
		expandingFile = filePath;
		generatingLineDiff = true;
		// Small delay to let UI update before expensive computation
		setTimeout(() => { generatingLineDiff = false; }, 100);
	}

	function computeSelectedFileDiffs(fileKey: string): Array<{ num: number; type: string; content: string }> {
		if (!diffData || !fileKey) return [{ num: 1, type: 'same', content: '(No file selected)' }];
		const item = diffData.diffItems?.find((d: any) => d.path === fileKey);
		if (!item) return [{ num: 1, type: 'same', content: '(File not found in diff)' }];
		if (item.content) return [{ num: 1, type: 'same', content: item.content.replace(/\n/g, '\n') }];

		const oldLines = (diffData.contents?.[item.versionA]?.[fileKey] || '').split('\n');
		const newLines = (diffData.contents?.[item.versionB]?.[fileKey] || '').split('\n');
		if (oldLines.length <= 1 && newLines.length <= 1) return [{ num: 1, type: 'same', content: '(No meaningful changes)' }];

		const result: Array<{ num: number; type: string; content: string }> = [];
		const m = oldLines.length, n = newLines.length;
		const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0));
		for (let i = m - 1; i >= 0; i--) {
			for (let j = n - 1; j >= 0; j--) {
				dp[i][j] = oldLines[i] === newLines[j] ? 1 + dp[i + 1][j + 1] : Math.max(dp[i + 1][j], dp[i][j + 1]);
			}
		}
		let num = 1, i = 0, j = 0;
		while (i < m || j < n) {
			if (i < m && j < n && oldLines[i] === newLines[j]) { result.push({ num: num++, type: 'same', content: oldLines[i] }); i++; j++; }
			else if (j < n && (i === m || dp[i][j + 1] >= dp[i + 1][j])) { result.push({ num: num++, type: 'add', content: newLines[j] }); j++; }
			else if (i < m) { result.push({ num: num++, type: 'del', content: oldLines[i] }); i++; }
		}
		return result;
	}

	function copyDiffToClipboard() {
		const lines = computeSelectedFileDiffs(selectedFile!);
		const text = lines.map(l => (l.type === 'add' ? '+ ' : l.type === 'del' ? '- ' : ' ') + l.content).join('\n');
		navigator.clipboard.writeText(text).catch(() => {});
		alert('已复制到剪贴板');
	}

	function hasChanges(item: any): boolean {
		return item.status !== 'unchanged' && ((item.added || 0) + (item.removed || 0)) > 0;
	}

	function sortedVersions() {
		return [...versions].sort((a, b) => {
			const va = parseSemver(a.version), vb = parseSemver(b.version);
			if (vb.major !== va.major) return vb.major - va.major;
			if (vb.minor !== va.minor) return vb.minor - va.minor;
			return (vb.patch || 0) - (va.patch || 0);
		});
	}
</script>

<div class="vm-page">
	<!-- Header -->
	<header class="vm-header">
		<h1 class="vm-title"><span class="material-symbols-outlined">history_toggle_off</span> 版本管理</h1>
		<p class="vm-subtitle">Compare module versions, review changes, and rollback</p>
	</header>

	<!-- Tabs -->
	<nav class="vm-tabs">
		{#each ['list', 'diff', 'rollback'] as tab}
			<button class={'vm-tab ' + (activeTab === tab ? 'active' : '')}" onclick={() => activeTab = tab}>
				<span class="tab-icon">{tab === 'list' ? 'inventory_2' : tab === 'diff' ? 'compare_arrows' : 'undo'}</span>
				<span class="tab-label">{tab === 'list' ? 'Version List' : tab === 'diff' ? 'Diff View' : 'Rollback'}</span>
			</button>
		{/each}
	</nav>

	<main class="vm-main">
		<!-- ========== TAB 1: LIST ========== -->
		{#if activeTab === 'list'}
			{#if loading}
				<div class="vm-empty"><span class="spin material-symbols-outlined">sync</span><p>Loading versions...</p></div>
			{:else if versions.length === 0}
				<div class="vm-empty"><span class="material-symbols-outlined">cloud_download</span><h3>No Versions Found</h3><p>Create a version snapshot to get started.</p></div>
			{:else}
				<div class="vm-list">{#each sortedVersions() as v}
					{@const tag = getVersionTag(v.version)}
					<article class="vm-card" class:focus={expandedId === v.id}>
						<div class="vm-card-body">
							<div class="vm-name-row">
								<code class="vm-ver">{v.version}</code>
								<span class="vm-badge" style="background:{tag.color}20;color:{tag.color}">{tag.badge}</span>
							</div>
							<div class="vm-meta">{timeAgo(v.created_at)}{v.file_count ? ` • ${v.file_count} files` : ''}{v.total_size ? ` • ${formatSize(v.total_size)}` : ''}</div>
							{#if v.changelog}<p class="vm-changelog">{v.changelog.slice(0, 120)}{v.changelog.length > 120 ? '...' : ''}</p>{/if}
						</div>
						<div class="vm-actions">
							<button class="btn btn-sm" onclick={() => { selectedVersionA = v.version; selectedVersionB = ''; activeTab = 'diff'; setTimeout(generateDiff, 200); }}>
								<span class="material-symbols-outlined sm">visibility</span>View
							</button>
							<button class="btn btn-sm btn-warn" onclick={() => initiateRollback(v.version)}>
								<span class="material-symbols-outlined sm">undo</span>Rollback
							</button>
						</div>
					</article>{/each}</div>
			{/if}
		{/if}

		<!-- ========== TAB 2: DIFF ========== -->
		{#if activeTab === 'diff'}
			<section class="vm-diff-panel">
				<!-- Selectors -->
				<div class="vm-diff-bar">
					<div class="sel-group">
						<label class="sel-label">Base Version</label>
						<select class="vm-select" bind:value={selectedVersionA}>
							<option value="">— Select —</option>
							{#each sortedVersions() as v}
								<option value="{v.version}" selected={v.version === selectedVersionA}>{v.version} {timeAgo(v.created_at)}</option>
							{/each}
						</select>
					</div>
					<span class="diff-arrow"><span class="material-symbols-outlined">arrow_forward_ios</span></span>
					<div class="sel-group">
						<label class="sel-label">Compare With</label>
						<select class="vm-select" bind:value={selectedVersionB}>
							<option value="">— Select —</option>
							{#each sortedVersions() as v}
								<option value="{v.version}" selected={v.version === selectedVersionB}>{v.version} {timeAgo(v.created_at)}</option>
							{/each}
						</select>
					</div>
					<button class="btn btn-primary" disabled={!selectedVersionA || !selectedVersionB || comparing}" onclick={generateDiff}>
						{comparing ? <><span class="spin material-symbols-outlined sm">sync</span> Generating...</> : <><span class="material-symbols-outlined sm">compare</span> Generate Diff</>}
					</button>
				</div>

				<!-- Diff Content -->
				{#if comparing}<div class="vm-empty"><span class="spin material-symbols-outlined">hourglass_empty</span><p>Generating diff...</p></div>
				{:else if diffData && diffData.diffItems?.length}
					<div class="vm-diff-body">
						<!-- Sidebar -->
						<aside class="vm-sidebar">
							<h3 class="sidebar-heading">Changed Files ({diffData.diffItems.length})</h3>
							{#each diffData.diffItems as item}
								<div class={'vm-file-item ' + (selectedFile === item.path ? 'active' : '')}" onclick={() => selectDiffFile(item.path)}>
									<span class="fs-icon">{item.status === 'added' ? '+' : item.status === 'removed' ? '-' : '~'}</span>
									<span class="fp">{item.path}</span>
									<span class="fc">{item.added ? `+${item.added}` : ''}{item.removed ? `-${item.removed}` : ''}</span>
								</div>
							{/each}
						</aside>

						<!-- Viewer -->
						<section class="vm-viewer">
							<div class="viewer-head">
								<h3 class="fv-title">{selectedFile ? selectedFile : 'Select a file to view diff'}</h3>
								{#if selectedFile}<button class="btn btn-sm" onclick={copyDiffToClipboard}><span class="material-symbols-outlined sm">content_copy</span> Copy</button>{/if}
							</div>
							{#if generatingLineDiff}<div class="vm-empty"><span class="spin material-symbols-outlined">sync</span><p>Analyzing lines...</p></div>
							{:else if selectedFile}
								<pre class="vm-lines">{#each computeSelectedFileDiffs(selectedFile) as line}<span class={'vm-line vm-line-' + line.type}>&nbsp;&nbsp;<span class="ln">{line.num}</span>{line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' '}{line.content}</span>{/each}</pre>
							{:else}
								<div class="vm-empty"><span class="material-symbols-outlined">description</span><p>Select a changed file from the sidebar</p></div>
							{/if}
						</section>
					</div>
				{:else}
					<div class="vm-empty"><span class="material-symbols-outlined">compare_arrows</span><p>Select two versions and generate a diff to see what changed.</p></div>
				{/if}
			</section>
		{/if}

		<!-- ========== TAB 3: ROLLBACK ========== -->
		{#if activeTab === 'rollback'}
			<section class="vm-rollback">
				<h2 class="section-heading">Version Rollback</h2>
				<p class="section-desc">Restore your module to a previous version. All changes after the selected version will be reverted.</p>

				{#if rollbackLoading}
					<div class="vm-status"><span class="spin material-symbols-outlined">hourglass_top</span><p>Rolling back to <strong>{rollbackTarget}</strong>...</p></div>
				{:else}
					<div class="sel-group-lg">
						<label for="rb-sel">Select target version:</label>
						<select id="rb-sel" class="vm-select-lg" bind:value={rollbackTarget}>
							<option value="">— Select a version —</option>
							{#each sortedVersions() as v}<option value="{v.version}">{v.version} — {timeAgo(v.created_at)} ({v.file_count} files)</option>{/each}
						</select>
					</div>

					{#if rollbackStatusMsg}<div class="vm-alert ok">{rollbackStatusMsg}</div>{/if}

					<div class="vm-warning"><span class="material-symbols-outlined wi">warning_amber</span><div><strong>Warning!</strong> This operation replaces all current project files with those from the selected version.<br>This action cannot be undone unless you have a backup snapshot.</div></div>

					<button class="btn btn-danger" disabled={!rollbackTarget}" onclick={() => performRollback(rollbackTarget)}>
						<span class="material-symbols-outlined">undo</span> Rollback to {rollbackTarget || 'version'}
					</button>
				{/if}
			</section>
		{/if}
	</main>

	<!-- Confirmation Modal -->
	{#if showRollbackConfirm}
		<div class="modal-backdrop" onclick={() => showRollbackConfirm = false}>
			<div class="modal-content" onclick={(e) => e.stopPropagation()}>
				<h3>Confirm Rollback</h3>
				<p>Are you sure you want to rollback to <strong>{rollbackTarget}</strong>?</p>
				<p class="warn-text">This action cannot be undone.</p>
				<div class="modal-btns">
					<button class="btn btn-cancel" onclick={() => showRollbackConfirm = false}>Cancel</button>
					<button class="btn btn-confirm" onclick={() => performRollback(rollbackTarget)}>Confirm Rollback</button>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
/* ========== Layout ========== */
.vm-page { display:flex; flex-direction:column; gap:12px; padding:0; height:100%; }
.vm-header { display:flex; align-items:center; gap:12px; padding-bottom:8px; border-bottom:1px solid #2d2d2d; }
.vm-title { font-size:22px; font-weight:600; color:#e0e0e0; margin:0; display:flex; align-items:center; gap:8px; }
.vm-subtitle { font-size:13px; color:#888; margin:0; }

/* ========== Tabs ========== */
.vm-tabs { display:flex; gap:4px; background:#1a1a1a; padding:4px; border-radius:12px; }
.vm-tab { display:flex; align-items:center; gap:6px; padding:8px 16px; border:none; background:transparent; color:#888; border-radius:8px; cursor:pointer; font-size:13px; transition:all .2s; }
.vm-tab:hover { background:#2d2d2d; color:#ccc; }
.vm-tab.active { background:#1a73e8; color:#fff; }

/* ========== Main Area ========== */
.vm-main { flex:1; overflow-y:auto; }

/* ========== Card List ========== */
.vm-list { display:flex; flex-direction:column; gap:8px; }
.vm-card { display:flex; justify-content:space-between; align-items:flex-start; padding:14px 16px; background:#1a1a1a; border:1px solid #2d2d2d; border-radius:12px; transition:border-color .2s; }
.vm-card:hover { border-color:#1a73e8; }
.vm-card:focus { outline:2px solid #1a73e8; outline-offset:-2px; }
.vm-card-body { flex:1; display:flex; flex-direction:column; gap:6px; min-width:0; }
.vm-name-row { display:flex; align-items:center; gap:8px; }
.vm-ver { font-family:'JetBrains Mono',monospace; font-size:15px; font-weight:600; color:#4fc3f7; }
.vm-badge { padding:2px 8px; border-radius:4px; font-size:11px; font-weight:600; }
.vm-meta { font-size:12px; color:#888; }
.vm-changelog { font-size:12px; color:#aaa; margin:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.vm-actions { display:flex; gap:6px; flex-shrink:0; margin-left:12px; }

/* ========== Buttons ========== */
.btn { display:inline-flex; align-items:center; gap:4px; padding:6px 12px; border:1px solid #2d2d2d; background:#2d2d2d; color:#ccc; border-radius:8px; cursor:pointer; font-size:13px; transition:all .2s; }
.btn:hover:not(:disabled) { background:#3d3d3d; border-color:#1a73e8; }
.btn:disabled { opacity:.4; cursor:not-allowed; }
.btn-primary { background:#1a73e8; border-color:#1a73e8; color:#fff; }
.btn-primary:hover:not(:disabled) { background:#1565c0; }
.btn-warn { border-color:#ef535033; color:#ef5350; }
.btn-warn:hover:not(:disabled) { background:#ef535022; }
.btn-danger { background:#ef5350; border-color:#ef5350; color:#fff; }
.btn-danger:hover:not(:disabled) { background:#d32f2f; }
.btn-sm { padding:5px 10px; font-size:12px; }
.btn-cancel { background:#2d2d2d; color:#ccc; }
.btn-confirm { background:#ef5350; border-color:#ef5350; color:#fff; }
.sm { font-size:18px !important; }

/* ========== Diff Panel ========== */
.vm-diff-panel { display:flex; flex-direction:column; gap:12px; }
.vm-diff-bar { display:flex; align-items:flex-end; gap:12px; padding:14px; background:#1a1a1a; border-radius:12px; border:1px solid #2d2d2d; flex-wrap:wrap; }
.sel-group { display:flex; flex-direction:column; gap:4px; flex:1; min-width:160px; }
.sel-label { font-size:11px; color:#888; text-transform:uppercase; letter-spacing:.5px; }
.vm-select, .vm-select-lg { padding:8px 12px; background:#0d0d0d; border:1px solid #2d2d2d; border-radius:8px; color:#e0e0e0; font-size:13px; cursor:pointer; }
.vm-select-lg { width:100%; padding:10px; }
.diff-arrow { padding-top:16px; color:#555; }
.sel-group-lg { width:100%; }

/* Diff body */
.vm-diff-body { display:flex; border-radius:12px; overflow:hidden; border:1px solid #2d2d2d; min-height:400px; }
.vm-sidebar { width:280px; min-width:280px; background:#1a1a1a; border-right:1px solid #2d2d2d; overflow-y:auto; }
.sidebar-heading { padding:14px 16px; font-size:12px; font-weight:600; color:#888; text-transform:uppercase; margin:0; border-bottom:1px solid #2d2d2d; }
.vm-file-item { display:flex; align-items:center; gap:8px; padding:8px 14px; cursor:pointer; border-bottom:1px solid #2d2d2d; transition:background .15s; font-size:12px; }
.vm-file-item:hover { background:#2d2d2d; }
.vm-file-item.active { background:#1a73e822; border-left:3px solid #1a73e8; }
.fs-icon { color:#888; width:16px; text-align:center; font-weight:bold; }
.fp { flex:1; font-family:'JetBrains Mono',monospace; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:#ccc; }
.fc { color:#888; font-size:11px; white-space:nowrap; }

.vm-viewer { flex:1; display:flex; flex-direction:column; background:#0d0d0d; }
.viewer-head { display:flex; justify-content:space-between; align-items:center; padding:10px 14px; background:#1a1a1a; border-bottom:1px solid #2d2d2d; }
.fv-title { font-family:'JetBrains Mono',monospace; font-size:13px; color:#4fc3f7; margin:0; }
.vm-lines { font-family:'JetBrains Mono',monospace; font-size:12px; overflow:auto; max-height:calc(100vh - 400px); flex:1; padding:0; margin:0; }
.vm-line { display:block; line-height:1.6; }
.ln { display:inline-block; width:40px; color:#444; text-align:right; margin-right:10px; user-select:none; }
.vm-line-add { background:#1b5e2022; color:#4caf50; }
.vm-line-del { background:#b71c1c22; color:#f44336; text-decoration:line-through; }
.vm-line-same { color:#aaa; }

/* ========== Rollback ========== */
.vm-rollback { max-width:560px; display:flex; flex-direction:column; gap:14px; padding:24px; background:#1a1a1a; border-radius:12px; border:1px solid #2d2d2d; }
.section-heading { font-size:18px; font-weight:600; color:#e0e0e0; margin:0; }
.section-desc { font-size:13px; color:#888; margin:0; }
.vm-alert { padding:10px 14px; background:#1b5e2022; border:1px solid #4caf5033; border-radius:8px; color:#4caf50; font-size:13px; }
.vm-warning { display:flex; gap:8px; padding:12px; background:#b71c1c15; border:1px solid #b71c1c44; border-radius:8px; color:#ff8a80; font-size:13px; line-height:1.5; }
.wi { color:#ff8a80; }
.vm-status { display:flex; flex-direction:column; align-items:center; gap:10px; padding:30px; color:#4fc3f7; }

/* ========== Empty State ========== */
.vm-empty { display:flex; flex-direction:column; align-items:center; justify-content:center; gap:10px; padding:60px 20px; color:#666; text-align:center; }
.spin { animation:spin 1s linear infinite; }
@keyframes spin { from { transform:rotate(0deg); } to { transform:rotate(360deg); } }

/* ========== Modal ========== */
.modal-backdrop { position:fixed; inset:0; background:rgba(0,0,0,.7); display:flex; align-items:center; justify-content:center; z-index:100; }
.modal-content { background:#1a1a1a; border:1px solid #2d2d2d; border-radius:16px; padding:24px; width:400px; max-width:90vw; }
.modal-content h3 { margin:0 0 12px; color:#e0e0e0; }
.modal-content p { margin:0 0 8px; color:#ccc; font-size:14px; }
.warn-text { color:#ff8a80 !important; }
.modal-btns { display:flex; gap:12px; justify-content:flex-end; margin-top:18px; }

/* ========== Responsive ========== */
@media (max-width:768px) {
	.vm-diff-body { flex-direction:column; }
	.vm-sidebar { width:100%; min-width:auto; max-height:200px; border-right:none; border-bottom:1px solid #2d2d2d; }
}
</style>
