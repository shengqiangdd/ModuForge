import sqlite3
import os

# Check if database file exists
db_path = 'moduforge.db'
if not os.path.exists(db_path):
    print(f"Database file not found: {db_path}")
    # Try to find the database
    for root, dirs, files in os.walk('.'):
        for file in files:
            if file.endswith('.db'):
                print(f"Found database: {os.path.join(root, file)}")
else:
    print(f"Database file found: {db_path}")
    print(f"File size: {os.path.getsize(db_path)} bytes")
    
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # Check tables
    cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
    tables = cursor.fetchall()
    print(f"\nTables in database: {len(tables)}")
    for table in tables:
        print(f"  - {table[0]}")
    
    # Check project_files table structure
    print("\n=== project_files table structure ===")
    cursor.execute("PRAGMA table_info(project_files)")
    columns = cursor.fetchall()
    for col in columns:
        print(f"  {col[1]} ({col[2]})")
    
    # Count records in project_files
    cursor.execute("SELECT COUNT(*) FROM project_files")
    count = cursor.fetchone()[0]
    print(f"\nTotal records in project_files: {count}")
    
    # Check if our project exists
    cursor.execute("SELECT COUNT(*) FROM project_files WHERE project_id = '1785249992652501794-1864'")
    project_count = cursor.fetchone()[0]
    print(f"Records for project 1785249992652501794-1864: {project_count}")
    
    # Check projects table
    cursor.execute("SELECT id, name FROM projects WHERE id = '1785249992652501794-1864'")
    project = cursor.fetchone()
    if project:
        print(f"\nProject found: {project[1]}")
    else:
        print("\nProject not found in projects table")
    
    conn.close()
