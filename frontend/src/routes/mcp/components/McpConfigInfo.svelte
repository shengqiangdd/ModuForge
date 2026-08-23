<div class="rounded-2xl border p-4 mb-6 text-sm overflow-hidden" style="background: var(--color-bg-elevated); border-color: var(--color-border)">
  <div class="flex items-start gap-3 min-w-0">
    <span class="material-symbols-outlined text-[20px] mt-0.5 flex-shrink-0" style="color: var(--color-primary)">info</span>
    <div class="space-y-1.5 min-w-0 overflow-hidden">
      <p class="font-medium" style="color: var(--color-text)">如何配置 MCP 服务器</p>
      <p class="break-words" style="color: var(--color-text-secondary)">① 通过下方「添加服务器」动态管理（持久化到数据库，立即生效，无需重启）；② 或用环境变量静态配置（需重启服务）：</p>
      <pre class="rounded-xl p-3 overflow-x-auto text-xs font-mono max-w-full" style="background: var(--color-surface); color: var(--color-text-secondary)"><code>MCP_SERVERS='[&#123;"name":"github","url":"http://host:8000/mcp","headers":&#123;"Authorization":"Bearer x"&#125;&#125;]'
MCP_SERVERS_FILE=/path/to/servers.json   # 或使用配置文件</code></pre>
      <p class="text-xs break-words" style="color: var(--color-text-muted)">配置后 MCP 工具会自动注册为 <code>mcp__&lt;server&gt;__&lt;tool&gt;</code> 技能，AI 助手可直接调用。</p>
    </div>
  </div>
</div>
