import { describe, it, expect } from 'vitest';
import {
	languages,
	getLanguageByFile,
	getLanguageCodeMirror,
	getSupportedExtensions,
	isSupportedFile
} from './languages';

describe('languages', () => {
	it('should have languages defined', () => {
		expect(languages).toBeDefined();
		expect(languages.length).toBeGreaterThan(0);
	});

	it('should get language by file name', () => {
		const lang = getLanguageByFile('test.sh');
		expect(lang).toBeDefined();
		expect(lang?.id).toBe('shell');
	});

	it('should get language by file name with path', () => {
		const lang = getLanguageByFile('/path/to/script.sh');
		expect(lang?.id).toBe('shell');
	});

	it('should get TypeScript language', () => {
		const lang = getLanguageByFile('app.ts');
		expect(lang?.id).toBe('typescript');
	});

	it('should get JSON language', () => {
		const lang = getLanguageByFile('config.json');
		expect(lang?.id).toBe('json');
	});

	it('should get properties language', () => {
		const lang = getLanguageByFile('module.prop');
		expect(lang?.id).toBe('properties');
	});

	it('should get Markdown language', () => {
		const lang = getLanguageByFile('README.md');
		expect(lang?.id).toBe('markdown');
	});

	it('should return undefined for unknown extension', () => {
		const lang = getLanguageByFile('test.xyz');
		expect(lang).toBeUndefined();
	});

	it('should get CodeMirror language path', () => {
		const path = getLanguageCodeMirror('shell');
		expect(path).toBe('@codemirror/lang-shell');
	});

	it('should get CodeMirror by name', () => {
		const path = getLanguageCodeMirror('JavaScript');
		expect(path).toBe('@codemirror/lang-javascript');
	});

	it('should get supported extensions', () => {
		const exts = getSupportedExtensions();
		expect(exts).toContain('.sh');
		expect(exts).toContain('.js');
		expect(exts).toContain('.json');
	});

	it('should check if file is supported', () => {
		expect(isSupportedFile('test.sh')).toBe(true);
		expect(isSupportedFile('test.js')).toBe(true);
		expect(isSupportedFile('test.xyz')).toBe(false);
	});
});
