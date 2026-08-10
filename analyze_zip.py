#!/usr/bin/env python3
"""Analyze the downloaded zip file."""

import zipfile
import os

zip_path = "/tmp/test_module_v2.zip"

print("=== Analyzing ZIP contents ===")
try:
    with zipfile.ZipFile(zip_path, 'r') as zip_ref:
        print("\nAll files in zip:")
        for info in zip_ref.infolist():
            print(f"  {info.filename} ({info.file_size} bytes)")
        
        print("\n=== Checking for webroot wrapper ===")
        has_webroot = any('webroot/' in info.filename for info in zip_ref.infolist())
        print(f"  Has webroot directory: {has_webroot}")
        
        if has_webroot:
            print("\n  Files under webroot/:")
            for info in zip_ref.infolist():
                if 'webroot/' in info.filename:
                    print(f"    {info.filename}")
        
        print("\n=== Checking for excluded files (should NOT be present) ===")
        excluded_patterns = [
            ('.build_cache/', 'Build cache'),
            ('src/', 'Source code'),
            ('tmp/', 'Temp files'),
            ('DESIGN_DOC.md', 'Design doc'),
            ('app/backend/', 'Backend source'),
            ('*.d', 'Debug symbols'),
            ('.cargo-', 'Cargo lock'),
        ]
        
        for pattern, desc in excluded_patterns:
            found = any(pattern in info.filename for info in zip_ref.infolist())
            status = "FOUND (BAD!)" if found else "excluded (GOOD)"
            print(f"  {desc} ({pattern}): {status}")
        
        print("\n=== Expected files (should be present) ===")
        expected = [
            'META-INF/com/google/android/update-binary',
            'module.prop',
            'service.sh',
            'customize.sh',
        ]
        
        all_files = [info.filename for info in zip_ref.infolist()]
        for exp in expected:
            found = exp in all_files
            status = "present (GOOD)" if found else "MISSING (BAD!)"
            print(f"  {exp}: {status}")

except FileNotFoundError:
    print(f"Error: File not found: {zip_path}")
except Exception as e:
    print(f"Error: {e}")
