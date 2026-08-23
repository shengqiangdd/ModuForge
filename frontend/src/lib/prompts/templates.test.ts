import { describe, it, expect } from 'vitest';
import {
	templates,
	searchTemplates,
	getTemplatesByCategory,
	getCategories,
	getTemplateById
} from './templates';
import { classifyIntent, getUITemplate, generateStructuredPrompt } from './intentClassifier';

describe('templates', () => {
	it('should have templates defined', () => {
		expect(templates).toBeDefined();
		expect(templates.length).toBeGreaterThan(0);
	});

	it('should have all required fields', () => {
		for (const template of templates) {
			expect(template.id).toBeDefined();
			expect(template.name).toBeDefined();
			expect(template.category).toBeDefined();
			expect(template.description).toBeDefined();
			expect(template.prompt).toBeDefined();
			expect(template.parameters).toBeDefined();
		}
	});

	it('should search templates by query', () => {
		const results = searchTemplates('电池');
		expect(results.length).toBeGreaterThan(0);
		expect(results.some((t) => t.name.includes('电池'))).toBe(true);
	});

	it('should search templates by category', () => {
		const results = searchTemplates('隐私');
		expect(results.length).toBeGreaterThan(0);
	});

	it('should return all templates for empty query', () => {
		const results = searchTemplates('');
		expect(results.length).toBe(templates.length);
	});

	it('should get templates by category', () => {
		const results = getTemplatesByCategory('隐私安全');
		expect(results.length).toBeGreaterThan(0);
		for (const t of results) {
			expect(t.category).toBe('隐私安全');
		}
	});

	it('should get all categories', () => {
		const categories = getCategories();
		expect(categories.length).toBeGreaterThan(0);
		expect(categories).toContain('隐私安全');
	});

	it('should get template by ID', () => {
		const template = getTemplateById('detect-root');
		expect(template).toBeDefined();
		expect(template?.name).toBe('检测Root');
	});

	it('should return undefined for unknown ID', () => {
		const template = getTemplateById('nonexistent');
		expect(template).toBeUndefined();
	});
});

describe('intentClassifier', () => {
	it('should classify generate intent', () => {
		expect(classifyIntent('创建一个电池监控模块')).toBe('generate');
		expect(classifyIntent('帮我生成一个Magisk模块')).toBe('generate');
		expect(classifyIntent('Create a new module')).toBe('generate');
	});

	it('should classify modify intent', () => {
		expect(classifyIntent('修改build.prop属性')).toBe('modify');
		expect(classifyIntent('调整CPU频率')).toBe('modify');
		expect(classifyIntent('Update the configuration')).toBe('modify');
	});

	it('should classify query intent', () => {
		expect(classifyIntent('查看模块状态')).toBe('query');
		expect(classifyIntent('什么是Root')).toBe('query');
		expect(classifyIntent('How to check device status')).toBe('query');
	});

	it('should return unknown for unclear input', () => {
		expect(classifyIntent('')).toBe('unknown');
		expect(classifyIntent('hello')).toBe('unknown');
	});

	it('should get UI template for generate intent', () => {
		const template = getUITemplate('generate');
		expect(template.type).toBe('generate');
		expect(template.fields.length).toBeGreaterThan(0);
	});

	it('should get UI template for modify intent', () => {
		const template = getUITemplate('modify');
		expect(template.type).toBe('modify');
		expect(template.fields.length).toBeGreaterThan(0);
	});

	it('should get UI template for query intent', () => {
		const template = getUITemplate('query');
		expect(template.type).toBe('query');
		expect(template.fields.length).toBeGreaterThan(0);
	});

	it('should get UI template for unknown intent', () => {
		const template = getUITemplate('unknown');
		expect(template.type).toBe('unknown');
		expect(template.fields.length).toBeGreaterThan(0);
	});
});

describe('generateStructuredPrompt', () => {
	it('should generate prompt for generate intent', () => {
		const formData = {
			moduleType: '服务模块',
			featureName: '电池监控',
			description: '监控电池状态并记录日志',
			targetAndroid: 'Android 8.0+',
			useGo: false
		};

		const prompt = generateStructuredPrompt('generate', formData);
		expect(prompt).toContain('电池监控');
		expect(prompt).toContain('服务模块');
		expect(prompt).toContain('Android 8.0+');
		expect(prompt).toContain('Shell');
	});

	it('should generate prompt for generate intent with Go', () => {
		const formData = {
			moduleType: '工具模块',
			featureName: 'Root检测',
			description: '检测设备Root状态',
			useGo: true
		};

		const prompt = generateStructuredPrompt('generate', formData);
		expect(prompt).toContain('Go');
	});

	it('should generate prompt for modify intent', () => {
		const formData = {
			targetModule: 'battery_monitor',
			modifyType: '优化性能',
			modifyContent: '减少CPU使用率',
			priority: '高'
		};

		const prompt = generateStructuredPrompt('modify', formData);
		expect(prompt).toContain('battery_monitor');
		expect(prompt).toContain('优化性能');
		expect(prompt).toContain('高');
	});

	it('should generate prompt for query intent', () => {
		const formData = {
			queryType: '模块状态',
			queryTarget: 'battery_monitor',
			outputFormat: 'JSON'
		};

		const prompt = generateStructuredPrompt('query', formData);
		expect(prompt).toContain('模块状态');
		expect(prompt).toContain('battery_monitor');
		expect(prompt).toContain('JSON');
	});

	it('should generate prompt for unknown intent', () => {
		const formData = {
			userInput: '帮我创建一个广告屏蔽模块'
		};

		const prompt = generateStructuredPrompt('unknown', formData);
		expect(prompt).toContain('广告屏蔽');
	});

	it('should handle empty form data', () => {
		const prompt = generateStructuredPrompt('generate', {});
		expect(prompt).toBeDefined();
		expect(typeof prompt).toBe('string');
	});
});
