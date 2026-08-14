import sqlite3

conn = sqlite3.connect('moduforge.db')
cursor = conn.cursor()

# Get all files for the project
cursor.execute("SELECT path, content FROM project_files WHERE project_id = '1785249992652501794-1864'")
files = cursor.fetchall()

print(f"=== AndroBoost-SmartTune Project (1785249992652501794-1864) ===")
print(f"Total files in database: {len(files)}")
print()

# Categorize files
categories = {
    'module_prop': [],
    'shell_scripts': [],
    'rust_source': [],
    'cpp_source': [],
    'go_source': [],
    'web_ui': [],
    'config': [],
    'docs': [],
    'other': []
}

for path, content in files:
    if path == 'module.prop':
        categories['module_prop'].append((path, content))
    elif path.endswith('.sh'):
        categories['shell_scripts'].append((path, content))
    elif path.startswith('src/rust/') and path.endswith('.rs'):
        categories['rust_source'].append((path, content))
    elif path.startswith('src/cpp/') and (path.endswith('.cpp') or path.endswith('.h')):
        categories['cpp_source'].append((path, content))
    elif path.startswith('src/go/') and path.endswith('.go'):
        categories['go_source'].append((path, content))
    elif path.endswith('.html') or path.endswith('.css') or path.endswith('.js'):
        categories['web_ui'].append((path, content))
    elif path.endswith('.json') or path.endswith('.txt') and 'config' in path.lower():
        categories['config'].append((path, content))
    elif path.endswith('.md'):
        categories['docs'].append((path, content))
    else:
        categories['other'].append((path, content))

print("=== File Categories ===")
for cat, files_list in categories.items():
    if files_list:
        print(f"\n{cat.upper()} ({len(files_list)} files):")
        for path, content in files_list:
            print(f"  - {path} ({len(content)} bytes)")

# Check DESIGN_DOC.md for requirements
print("\n=== DESIGN DOCUMENT REQUIREMENTS ===")
for path, content in files:
    if path == 'DESIGN_DOC.md':
        print(content[:3000])
        break

# Check module.prop
print("\n=== MODULE.PROPERTIES ===")
for path, content in files:
    if path == 'module.prop':
        print(content)
        break

conn.close()
