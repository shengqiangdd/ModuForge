#!/usr/bin/env python3
"""Fix Unicode characters in deploy_now.py"""
with open('deploy_now.py', 'r', encoding='utf-8') as f:
    content = f.read()
content = content.replace('\u2713', '[OK]')
content = content.replace('\u2717', '[FAIL]')
with open('deploy_now.py', 'w', encoding='utf-8') as f:
    f.write(content)
print('Fixed Unicode characters')
