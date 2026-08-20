package service

func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		sessions:    make(map[string]*Conversation),
		maxSessions: 50,
		maxMessages: 200,
	}
}

func (cs *ConversationStore) Add(sessionID string, messages []Message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		if len(cs.sessions) >= cs.maxSessions {
			oldest := cs.order[0]
			delete(cs.sessions, oldest)
			cs.order = cs.order[1:]
		}
		conv = &Conversation{}
		cs.sessions[sessionID] = conv
		cs.order = append(cs.order, sessionID)
	}

	if len(messages) > cs.maxMessages {
		messages = messages[len(messages)-cs.maxMessages:]
	}
	conv.Messages = messages
}

func (cs *ConversationStore) Get(sessionID string) []Message {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		return nil
	}
	result := make([]Message, len(conv.Messages))
	copy(result, conv.Messages)
	return result
}

func (cs *ConversationStore) Delete(sessionID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.sessions[sessionID]; !ok {
		return
	}
	delete(cs.sessions, sessionID)
	// Remove from order slice
	for i, id := range cs.order {
		if id == sessionID {
			cs.order = append(cs.order[:i], cs.order[i+1:]...)
			return
		}
	}
}

func (cs *ConversationStore) Append(sessionID string, msgs ...Message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		if len(cs.sessions) >= cs.maxSessions {
			oldest := cs.order[0]
			delete(cs.sessions, oldest)
			cs.order = cs.order[1:]
		}
		conv = &Conversation{}
		cs.sessions[sessionID] = conv
		cs.order = append(cs.order, sessionID)
	}

	conv.Messages = append(conv.Messages, msgs...)
	if len(conv.Messages) > cs.maxMessages {
		excess := len(conv.Messages) - cs.maxMessages
		conv.Messages = conv.Messages[excess:]
	}
}
