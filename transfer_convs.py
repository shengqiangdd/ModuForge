"""Try to reassign conversations from admin to csq via API"""
import urllib.request, json

BASE = 'http://192.168.2.9:8086'
CSQ_ID = 'a4c50d84-a58d-4fbc-a64d-adf93ca14446'
ADMIN_ID = 'fec17bd3-7610-4f2a-b157-24ee1e362d23'

# Login as admin
data = json.dumps({'username': 'admin', 'password': 'admin123'}).encode()
req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
admin_token = json.loads(resp.read())['token']
admin_headers = {'Authorization': f'Bearer {admin_token}'}

# Get admin's conversations
req = urllib.request.Request(f'{BASE}/api/v1/ai/conversations', headers=admin_headers)
resp = urllib.request.urlopen(req)
convs = json.loads(resp.read())
conv_list = convs.get('conversations', convs) if isinstance(convs, dict) else convs

print(f'Admin has {len(conv_list)} conversations')

# Try to get full conversation details and recreate for csq
data2 = json.dumps({'username': 'csq', 'password': 'csq0216'}).encode()
req2 = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data2, headers={'Content-Type': 'application/json'})
resp2 = urllib.request.urlopen(req2)
csq_token = json.loads(resp2.read())['token']
csq_headers = {'Authorization': f'Bearer {csq_token}', 'Content-Type': 'application/json'}

for cv in conv_list:
    cv_id = cv.get('id')
    title = cv.get('title', '?')
    
    # Get full conversation with messages
    try:
        req3 = urllib.request.Request(f'{BASE}/api/v1/ai/conversations/{cv_id}', headers=admin_headers)
        resp3 = urllib.request.urlopen(req3)
        full_cv = json.loads(resp3.read())
        messages = full_cv.get('messages', [])
        project_id = full_cv.get('project_id')
        mode = full_cv.get('mode', 'agent')
        
        # Create conversation for csq
        create_data = json.dumps({
            'title': title,
            'project_id': project_id,
            'mode': mode
        }).encode()
        
        req4 = urllib.request.Request(f'{BASE}/api/v1/ai/conversations', data=create_data, headers=csq_headers)
        resp4 = urllib.request.urlopen(req4)
        new_cv = json.loads(resp4.read())
        new_id = new_cv.get('id')
        print(f'  Created: {title[:40]}... (msgs={len(messages)}, new_id={new_id})')
        
        # Send each message to the new conversation
        for msg in messages:
            role = msg.get('role', 'user')
            content = msg.get('content', '')
            if role == 'user' and content:
                send_data = json.dumps({
                    'message': content,
                    'project_id': project_id,
                    'conversation_id': new_id
                }).encode()
                try:
                    req5 = urllib.request.Request(f'{BASE}/api/v1/ai/chat', data=send_data, headers=csq_headers)
                    resp5 = urllib.request.urlopen(req5)
                except:
                    pass  # Some messages may not be sendable
                    
    except Exception as e:
        print(f'  Failed: {title[:40]}... - {e}')

print('\nDone!')
