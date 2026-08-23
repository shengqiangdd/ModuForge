export type Intent = 'generate' | 'modify' | 'query' | 'unknown';

export interface UITemplate {
	type: Intent;
	title: string;
	description: string;
	fields: UIField[];
}

export interface UIField {
	name: string;
	label: string;
	type: 'text' | 'textarea' | 'number' | 'select' | 'checkbox';
	placeholder?: string;
	options?: string[];
	required?: boolean;
	defaultValue?: string | number | boolean;
}

/**
 * Classify user input intent based on keyword rules
 */
export function classifyIntent(input: string): Intent {
	const lower = input.toLowerCase();

	// Generate patterns
	const generatePatterns = [
		'创建', '生成', '制作', '写一个', '帮我', '开发', '构建', '设计',
		'create', 'generate', 'build', 'make', 'develop', 'design', 'write'
	];

	// Modify patterns
	const modifyPatterns = [
		'修改', '更改', '调整', '优化', '更新', '编辑', '改', '调整',
		'modify', 'change', 'update', 'edit', 'adjust', 'optimize', 'tweak'
	];

	// Query patterns
	const queryPatterns = [
		'什么是', '如何', '怎么', '为什么', '能否', '可以', '查询', '查看',
		'what', 'how', 'why', 'can', 'could', 'query', 'check', 'show', 'list'
	];

	const scores: Record<Intent, number> = {
		generate: 0,
		modify: 0,
		query: 0,
		unknown: 0
	};

	// Score based on patterns
	for (const pattern of generatePatterns) {
		if (lower.includes(pattern)) scores.generate += 1;
	}

	for (const pattern of modifyPatterns) {
		if (lower.includes(pattern)) scores.modify += 1;
	}

	for (const pattern of queryPatterns) {
		if (lower.includes(pattern)) scores.query += 1;
	}

	// Boost patterns based on context
	if (lower.includes('模块') || lower.includes('module')) {
		scores.generate += 0.5;
	}

	if (lower.includes('属性') || lower.includes('prop')) {
		scores.modify += 0.5;
	}

	if (lower.includes('状态') || lower.includes('status')) {
		scores.query += 0.5;
	}

	// Find highest score
	let maxScore = 0;
	let bestIntent: Intent = 'unknown';

	for (const [intent, score] of Object.entries(scores) as [Intent, number][]) {
		if (score > maxScore) {
			maxScore = score;
			bestIntent = intent;
		}
	}

	// Return unknown if no patterns matched
	if (maxScore === 0) {
		return 'unknown';
	}

	return bestIntent;
}

/**
 * Get UI template configuration based on intent
 */
export function getUITemplate(intent: Intent): UITemplate {
	switch (intent) {
		case 'generate':
			return {
				type: 'generate',
				title: '创建新模块',
				description: '描述你想要的功能，系统将生成对应的Magisk模块',
				fields: [
					{
						name: 'moduleType',
						label: '模块类型',
						type: 'select',
						options: ['服务模块', '工具模块', '修改模块', '调整模块'],
						required: true,
						defaultValue: '服务模块'
					},
					{
						name: 'featureName',
						label: '功能名称',
						type: 'text',
						placeholder: '例如：电池监控、性能优化',
						required: true
					},
					{
						name: 'description',
						label: '详细描述',
						type: 'textarea',
						placeholder: '描述模块需要实现的功能...',
						required: true
					},
					{
						name: 'targetAndroid',
						label: '目标Android版本',
						type: 'select',
						options: ['Android 8.0+', 'Android 10+', 'Android 12+', 'Android 14+'],
						defaultValue: 'Android 8.0+'
					},
					{
						name: 'useGo',
						label: '使用Go语言',
						type: 'checkbox',
						defaultValue: false
					}
				]
			};

		case 'modify':
			return {
				type: 'modify',
				title: '修改现有模块',
				description: '选择要修改的模块和修改内容',
				fields: [
					{
						name: 'targetModule',
						label: '目标模块',
						type: 'select',
						options: ['当前项目', '指定模块'],
						required: true
					},
					{
						name: 'modifyType',
						label: '修改类型',
						type: 'select',
						options: ['添加功能', '修复Bug', '优化性能', '调整参数'],
						required: true
					},
					{
						name: 'modifyContent',
						label: '修改内容',
						type: 'textarea',
						placeholder: '描述需要修改的内容...',
						required: true
					},
					{
						name: 'priority',
						label: '优先级',
						type: 'select',
						options: ['高', '中', '低'],
						defaultValue: '中'
					}
				]
			};

		case 'query':
			return {
				type: 'query',
				title: '查询信息',
				description: '查询模块、设备或系统状态',
				fields: [
					{
						name: 'queryType',
						label: '查询类型',
						type: 'select',
						options: ['模块状态', '设备信息', '系统属性', '构建日志'],
						required: true
					},
					{
						name: 'queryTarget',
						label: '查询目标',
						type: 'text',
						placeholder: '例如：battery_monitor, /system/build.prop',
						required: true
					},
					{
						name: 'outputFormat',
						label: '输出格式',
						type: 'select',
						options: ['文本', 'JSON', '表格'],
						defaultValue: '文本'
					}
				]
			};

		default:
			return {
				type: 'unknown',
				title: '智能助手',
				description: '请描述你的需求，系统将自动识别并提供建议',
				fields: [
					{
						name: 'userInput',
						label: '你的需求',
						type: 'textarea',
						placeholder: '例如：我想创建一个电池监控模块...',
						required: true
					}
				]
			};
	}
}

/**
 * Generate a structured prompt from form data
 */
export function generateStructuredPrompt(intent: Intent, formData: Record<string, unknown>): string {
	switch (intent) {
		case 'generate': {
			const moduleType = formData.moduleType || '服务模块';
			const featureName = formData.featureName || '';
			const description = formData.description || '';
			const targetAndroid = formData.targetAndroid || 'Android 8.0+';
			const useGo = formData.useGo || false;

			return `创建一个Magisk ${moduleType}，功能：${featureName}。

要求：
${description}

技术约束：
- 目标系统：${targetAndroid}
- 编程语言：${useGo ? 'Go' : 'Shell'}
- 需要生成完整的模块结构（module.prop, customize.sh, service.sh等）

请生成完整的模块代码。`;
		}

		case 'modify': {
			const targetModule = formData.targetModule || '当前项目';
			const modifyType = formData.modifyType || '添加功能';
			const modifyContent = formData.modifyContent || '';
			const priority = formData.priority || '中';

			return `修改${targetModule}，操作：${modifyType}。

修改内容：
${modifyContent}

优先级：${priority}

请修改对应的文件并确保修改后模块仍能正常工作。`;
		}

		case 'query': {
			const queryType = formData.queryType || '模块状态';
			const queryTarget = formData.queryTarget || '';
			const outputFormat = formData.outputFormat || '文本';

			return `查询${queryType}信息。

目标：${queryTarget}
输出格式：${outputFormat}

请提供查询结果。`;
		}

		default: {
			const userInput = formData.userInput || '';
			return userInput;
		}
	}
}
