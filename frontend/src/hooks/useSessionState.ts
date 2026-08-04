// useSessionState.ts - Hook for managing session state persistence
import { useState, useEffect, useCallback } from 'react';

interface SessionState {
  sessionId: string;
  projectId: string;
  mode: 'plan' | 'act';
  toolsEnabled: Record<string, boolean>;
  preferences: Record<string, any>;
  checkpoints: Array<{
    path: string;
    content: string;
    timestamp: number;
  }>;
  toolHistory: Record<string, number>;
}

const STORAGE_KEY = 'moduforge_session_state';

export const useSessionState = (sessionId: string) => {
  const [state, setState] = useState<SessionState>({
    sessionId,
    projectId: '',
    mode: 'act',
    toolsEnabled: {},
    preferences: {},
    checkpoints: [],
    toolHistory: {},
  });

  // Load from localStorage on mount
  useEffect(() => {
    try {
      const saved = localStorage.getItem(`${STORAGE_KEY}_${sessionId}`);
      if (saved) {
        const parsed = JSON.parse(saved);
        setState((prev) => ({ ...prev, ...parsed, sessionId }));
      }
    } catch (e) {
      console.error('Failed to load session state:', e);
    }
  }, [sessionId]);

  // Save to localStorage on state change
  useEffect(() => {
    try {
      localStorage.setItem(
        `${STORAGE_KEY}_${sessionId}`,
        JSON.stringify(state)
      );
    } catch (e) {
      console.error('Failed to save session state:', e);
    }
  }, [state, sessionId]);

  const setMode = useCallback((mode: 'plan' | 'act') => {
    setState((prev) => ({ ...prev, mode }));
  }, []);

  const setProjectId = useCallback((projectId: string) => {
    setState((prev) => ({ ...prev, projectId }));
  }, []);

  const toggleTool = useCallback((toolName: string) => {
    setState((prev) => ({
      ...prev,
      toolsEnabled: {
        ...prev.toolsEnabled,
        [toolName]: !prev.toolsEnabled[toolName],
      },
    }));
  }, []);

  const addCheckpoint = useCallback(
    (path: string, content: string) => {
      setState((prev) => ({
        ...prev,
        checkpoints: [
          ...prev.checkpoints.slice(-19), // Keep last 20
          { path, content, timestamp: Date.now() },
        ],
      }));
    },
    []
  );

  const removeCheckpoint = useCallback((path: string) => {
    setState((prev) => ({
      ...prev,
      checkpoints: prev.checkpoints.filter((cp) => cp.path !== path),
    }));
  }, []);

  const recordToolCall = useCallback((toolName: string) => {
    setState((prev) => ({
      ...prev,
      toolHistory: {
        ...prev.toolHistory,
        [toolName]: (prev.toolHistory[toolName] || 0) + 1,
      },
    }));
  }, []);

  const setPreference = useCallback((key: string, value: any) => {
    setState((prev) => ({
      ...prev,
      preferences: { ...prev.preferences, [key]: value },
    }));
  }, []);

  const clearSession = useCallback(() => {
    localStorage.removeItem(`${STORAGE_KEY}_${sessionId}`);
    setState({
      sessionId,
      projectId: '',
      mode: 'act',
      toolsEnabled: {},
      preferences: {},
      checkpoints: [],
      toolHistory: {},
    });
  }, [sessionId]);

  return {
    state,
    setMode,
    setProjectId,
    toggleTool,
    addCheckpoint,
    removeCheckpoint,
    recordToolCall,
    setPreference,
    clearSession,
  };
};

export default useSessionState;
