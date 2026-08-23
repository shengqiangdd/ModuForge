export interface TemplateParameter {
	name: string;
	label: string;
	type: 'string' | 'number' | 'boolean' | 'select';
	required: boolean;
	defaultValue?: string | number | boolean;
	options?: string[];
	description?: string;
}

export interface PromptTemplate {
	id: string;
	name: string;
	category: string;
	description: string;
	prompt: string;
	parameters: TemplateParameter[];
}

export const templates: PromptTemplate[] = [
	{
		id: 'detect-root',
		name: '检测Root',
		category: '系统工具',
		description: '检测设备是否已获取Root权限，支持多种Root方案检测',
		prompt: `创建一个Magisk模块，检测设备Root状态。
功能要求：
1. 检测Magisk是否安装（检查${0}路径）
2. 检测KernelSU是否安装
3. 检测APatch是否安装
4. 检测SuperSU是否安装
5. 输出检测结果到${1}
6. 支持${2}格式输出`,
		parameters: [
			{ name: 'checkPath', label: '检测路径', type: 'string', required: true, defaultValue: '/data/adb/magisk', description: 'Magisk安装路径' },
			{ name: 'outputPath', label: '输出路径', type: 'string', required: true, defaultValue: '/data/local/tmp/root_status.txt', description: '结果输出文件' },
			{ name: 'outputFormat', label: '输出格式', type: 'select', required: true, defaultValue: 'text', options: ['text', 'json', 'xml'], description: '结果格式' }
		]
	},
	{
		id: 'hide-app',
		name: '隐藏应用',
		category: '隐私安全',
		description: '隐藏指定应用的Root检测和Magisk痕迹',
		prompt: `创建一个Magisk模块，隐藏指定应用。
功能要求：
1. 使用Zygisk或Shamiko方案隐藏${0}
2. 隐藏Magisk应用本身（包名：${1}）
3. 隐藏su二进制文件
4. 隐藏Magisk Manager
5. 支持${2}列表配置
6. ${3}模式：完全隐藏/部分隐藏`,
		parameters: [
			{ name: 'targetApps', label: '目标应用', type: 'string', required: true, description: '要隐藏的应用包名（逗号分隔）' },
			{ name: 'magiskPackage', label: 'Magisk包名', type: 'string', required: false, defaultValue: 'com.topjohnwu.magisk', description: 'Magisk Manager包名' },
			{ name: 'scopeList', label: '作用域列表', type: 'string', required: false, description: '应用作用域配置' },
			{ name: 'hideMode', label: '隐藏模式', type: 'select', required: true, defaultValue: 'full', options: ['full', 'partial'], description: '隐藏程度' }
		]
	},
	{
		id: 'modify-buildprop',
		name: '修改系统属性',
		category: '系统定制',
		description: '修改build.prop系统属性，自定义设备信息',
		prompt: `创建一个Magisk模块，修改系统属性。
功能要求：
1. 在${0}中添加以下属性：
${1}
2. 备份原始build.prop到${2}
3. 支持${3}恢复
4. 属性修改前检查兼容性
5. 记录所有修改到${4}`,
		parameters: [
			{ name: 'targetFile', label: '目标文件', type: 'select', required: true, defaultValue: '/system/build.prop', options: ['/system/build.prop', '/vendor/build.prop'], description: '要修改的属性文件' },
			{ name: 'properties', label: '属性列表', type: 'string', required: true, description: '要添加/修改的属性（key=value格式，每行一个）' },
			{ name: 'backupPath', label: '备份路径', type: 'string', required: false, defaultValue: '/data/adb/modules/backup', description: '备份文件位置' },
			{ name: 'bootRestore', label: '启动恢复', type: 'boolean', required: false, defaultValue: true, description: '启动时自动恢复' },
			{ name: 'logPath', label: '日志路径', type: 'string', required: false, defaultValue: '/data/local/tmp/prop_changes.log', description: '修改日志位置' }
		]
	},
	{
		id: 'performance-optimize',
		name: '性能优化',
		category: '性能调优',
		description: 'CPU/GPU调度优化，提升设备性能',
		prompt: `创建一个Magisk模块，优化设备性能。
功能要求：
1. CPU调度优化：
   - 设置${0}调度器
   - 调整${1}频率范围
   - 优化${2}核心策略
2. GPU优化：
   - 设置GPU频率${3}
   - 启用${4}模式
3. 内存优化：
   - 调整${5}
   - 配置${6}
4. 性能监控：${7}`,
		parameters: [
			{ name: 'scheduler', label: 'CPU调度器', type: 'select', required: true, defaultValue: 'schedutil', options: ['schedutil', 'interactive', 'ondemand', 'performance'], description: 'CPU调度策略' },
			{ name: 'cpuFreq', label: 'CPU频率', type: 'string', required: true, description: '频率范围（min-max）' },
			{ name: 'corePolicy', label: '核心策略', type: 'select', required: true, defaultValue: 'balanced', options: ['balanced', 'performance', 'powersave'], description: '核心使用策略' },
			{ name: 'gpuFreq', label: 'GPU频率', type: 'string', required: false, description: 'GPU频率范围' },
			{ name: 'gpuMode', label: 'GPU模式', type: 'select', required: false, defaultValue: 'adaptive', options: ['adaptive', 'performance', 'powersave'], description: 'GPU工作模式' },
			{ name: 'vmSettings', label: '虚拟内存', type: 'string', required: false, description: 'vm参数设置' },
			{ name: 'ioScheduler', label: 'IO调度器', type: 'select', required: false, defaultValue: 'bfq', options: ['bfq', 'cfq', 'deadline', 'noop'], description: 'IO调度策略' },
			{ name: 'enableMonitor', label: '启用监控', type: 'boolean', required: false, defaultValue: true, description: '启用性能监控日志' }
		]
	},
	{
		id: 'battery-manage',
		name: '电池管理',
		category: '电池优化',
		description: '电池监控、省电策略和充电管理',
		prompt: `创建一个Magisk模块，管理电池。
功能要求：
1. 电池监控：
   - 监控${0}
   - 记录${1}
   - 检测${2}
2. 省电策略：
   - 当电量低于${3}%时启用省电模式
   - ${4}自动调整CPU频率
3. 充电管理：
   - ${5}充电阈值
   - ${6}充电保护
4. 通知：${7}`,
		parameters: [
			{ name: 'monitorPath', label: '监控路径', type: 'string', required: true, defaultValue: '/sys/class/power_supply/battery', description: '电池信息路径' },
			{ name: 'logInterval', label: '记录间隔', type: 'number', required: false, defaultValue: 60, description: '记录间隔（秒）' },
			{ name: 'healthCheck', label: '健康检测', type: 'boolean', required: false, defaultValue: true, description: '启用电池健康检测' },
			{ name: 'lowBattery', label: '低电量阈值', type: 'number', required: false, defaultValue: 20, description: '低电量阈值（%）' },
			{ name: 'autoAdjust', label: '自动调整', type: 'boolean', required: false, defaultValue: true, description: '自动调整CPU频率' },
			{ name: 'chargeLimit', label: '充电限制', type: 'number', required: false, defaultValue: 80, description: '充电上限（%）' },
			{ name: 'chargeProtect', label: '充电保护', type: 'boolean', required: false, defaultValue: true, description: '启用充电保护' },
			{ name: 'notifyEnabled', label: '启用通知', type: 'boolean', required: false, defaultValue: true, description: '启用通知提醒' }
		]
	},
	{
		id: 'ad-block',
		name: '广告屏蔽',
		category: '隐私安全',
		description: '系统级广告拦截，支持多种屏蔽方式',
		prompt: `创建一个Magisk模块，屏蔽广告。
功能要求：
1. hosts屏蔽：
   - 加载${0}
   - 屏蔽${1}域名
2. iptables屏蔽：
   - 阻止${2}端口
   - 重定向${3}
3. 应用层屏蔽：
   - 屏蔽${4}
   - 拦截${5}
4. 白名单：${6}
5. 日志：${7}`,
		parameters: [
			{ name: 'hostsFile', label: 'Hosts文件', type: 'string', required: true, defaultValue: '/system/etc/hosts', description: 'hosts文件路径' },
			{ name: 'blockDomains', label: '屏蔽域名', type: 'string', required: true, description: '要屏蔽的域名列表' },
			{ name: 'blockPorts', label: '屏蔽端口', type: 'string', required: false, defaultValue: '80,443', description: '要屏蔽的端口' },
			{ name: 'redirectUrl', label: '重定向地址', type: 'string', required: false, defaultValue: '0.0.0.0', description: '重定向目标地址' },
			{ name: 'appAds', label: '应用广告', type: 'boolean', required: false, defaultValue: true, description: '屏蔽应用内广告' },
			{ name: 'trackingIds', label: '追踪ID', type: 'string', required: false, description: '要屏蔽的追踪ID' },
			{ name: 'whitelist', label: '白名单', type: 'string', required: false, description: '白名单域名' },
			{ name: 'logPath', label: '日志路径', type: 'string', required: false, defaultValue: '/data/local/tmp/adblock.log', description: '屏蔽日志' }
		]
	}
];

/**
 * Fuzzy search templates by query
 */
export function searchTemplates(query: string): PromptTemplate[] {
	if (!query || query.trim() === '') return templates;

	const lowerQuery = query.toLowerCase();
	const terms = lowerQuery.split(/\s+/).filter((t) => t.length > 0);

	return templates
		.map((template) => {
			let score = 0;
			const searchText = `${template.name} ${template.category} ${template.description}`.toLowerCase();

			for (const term of terms) {
				if (searchText.includes(term)) {
					score += 1;
				}
				if (template.name.toLowerCase().includes(term)) {
					score += 2;
				}
				if (template.category.toLowerCase().includes(term)) {
					score += 1.5;
				}
			}

			return { template, score };
		})
		.filter((item) => item.score > 0)
		.sort((a, b) => b.score - a.score)
		.map((item) => item.template);
}

/**
 * Get templates by category
 */
export function getTemplatesByCategory(category: string): PromptTemplate[] {
	return templates.filter((t) => t.category === category);
}

/**
 * Get all unique categories
 */
export function getCategories(): string[] {
	const categories = new Set(templates.map((t) => t.category));
	return Array.from(categories);
}

/**
 * Get template by ID
 */
export function getTemplateById(id: string): PromptTemplate | undefined {
	return templates.find((t) => t.id === id);
}
