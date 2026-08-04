// ProgressIndicator.tsx - Tool call progress visualization
import React from 'react';

interface ToolCall {
  id: string;
  name: string;
  status: 'pending' | 'running' | 'success' | 'error';
  input?: Record<string, any>;
  output?: string;
  duration?: number;
  timestamp?: number;
}

interface ProgressIndicatorProps {
  toolCalls: ToolCall[];
  currentStep?: string;
  iteration?: number;
  maxIterations?: number;
}

// Tool icons mapping
const toolIcons: Record<string, string> = {
  read_file: '📖',
  write_file: '✏️',
  edit_file: '📝',
  bash: '💻',
  grep_search: '🔍',
  glob_search: '📂',
  build_module: '🔨',
  create_module: '📦',
  think: '🤔',
  list_dir: '📁',
  detect: '🔎',
  web_search: '🌐',
  generate_code: '🤖',
};

// Status colors
const statusColors: Record<string, string> = {
  pending: '#9ca3af',
  running: '#3b82f6',
  success: '#10b981',
  error: '#ef4444',
};

export const ProgressIndicator: React.FC<ProgressIndicatorProps> = ({
  toolCalls,
  currentStep,
  iteration,
  maxIterations,
}) => {
  if (toolCalls.length === 0) return null;

  return (
    <div className="progress-container">
      {/* Overall progress bar */}
      {iteration !== undefined && maxIterations !== undefined && (
        <div className="progress-bar-container">
          <div className="progress-bar-label">
            <span>迭代进度</span>
            <span>{iteration}/{maxIterations}</span>
          </div>
          <div className="progress-bar">
            <div
              className="progress-bar-fill"
              style={{
                width: `${(iteration / maxIterations) * 100}%`,
                backgroundColor: iteration >= maxIterations * 0.8 ? '#f59e0b' : '#3b82f6',
              }}
            />
          </div>
        </div>
      )}

      {/* Tool calls timeline */}
      <div className="tool-timeline">
        {toolCalls.map((tc, idx) => (
          <div key={tc.id || idx} className={`tool-call-item ${tc.status}`}>
            <div className="tool-call-icon">
              {toolIcons[tc.name] || '🔧'}
            </div>
            <div className="tool-call-info">
              <div className="tool-call-name">{tc.name}</div>
              {tc.input?.path && (
                <div className="tool-call-path">{tc.input.path}</div>
              )}
              {tc.duration !== undefined && (
                <div className="tool-call-duration">{tc.duration}ms</div>
              )}
            </div>
            <div
              className="tool-call-status"
              style={{ color: statusColors[tc.status] }}
            >
              {tc.status === 'running' && (
                <span className="spinner">⏳</span>
              )}
              {tc.status === 'success' && '✅'}
              {tc.status === 'error' && '❌'}
              {tc.status === 'pending' && '⏸️'}
            </div>
          </div>
        ))}
      </div>

      {/* Current step indicator */}
      {currentStep && (
        <div className="current-step">
          <span className="spinner">⏳</span>
          <span>{currentStep}</span>
        </div>
      )}
    </div>
  );
};

export default ProgressIndicator;
