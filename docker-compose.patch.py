import re
with open('docker-compose.yml') as f: c = f.read
c = c.replace('image: moduforge:latest', 'image: moduforge:patched')
with open('docker-compose.yml', 'w') as f: f.write(c)
