// UndoRedoPanel.tsx - Undo/Redo history panel for file changes
import React, { useState, useEffect } from 'react';

interface Checkpoint {
  path: string;
  content: string;
  timestamp: number;
  action: 'write' | 'edit' | 'delete';
}

interface UndoRedoPanelProps {
  checkpoints: Checkpoint[];
  onUndo: (checkpoint: Checkpoint) => void;
  onRedo: (checkpoint: Checkpoint) => void;
  currentVersion?: number;
}

export const UndoRedoPanel: React.FC<UndoRedoPanelProps> = ({
  checkpoints,
  onUndo,
  onRedo,
  currentVersion,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null);

  if (checkpoints.length === 0) return null;

  const formatTime = (ts: number) => {
    const d = new Date(ts);
    return d.toLocaleTimeString();
  };

  const getActionIcon = (action: string) => {
    switch (action) {
      case 'write': return '📝';
      case 'edit': return '✏️';
      case 'delete': return '🗑️';
      default: return '📄';
    }
  };

  return (
    <div className="undo-redo-container">
      <button
        className="undo-redo-toggle"
        onClick={() => setIsOpen(!isOpen)}
        title="查看修改历史"
      >
        🔄 历史 ({checkpoints.length})
      </button>

      {isOpen && (
        <div className="undo-redo-panel">
          <div className="panel-header">
            <h4>修改历史</h4>
            <button onClick={() => setIsOpen(false)}>×</button>
          </div>
          <div className="checkpoint-list">
            {checkpoints.map((cp, idx) => (
              <div
                key={idx}
                className={`checkpoint-item ${selectedIdx === idx ? 'selected' : ''}`}
                onClick={() => setSelectedIdx(idx)}
              >
                <div className="checkpoint-icon">
                  {getActionIcon(cp.action)}
                </div>
                <div className="checkpoint-info">
                  <div className="checkpoint-path">{cp.path}</div>
                  <div className="checkpoint-time">{formatTime(cp.timestamp)}</div>
                </div>
                <div className="checkpoint-actions">
                  {idx < checkpoints.length - 1 && (
                    <button
                      className="undo-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        onUndo(cp);
                      }}
                      title="撤销此修改"
                    >
                      ↩️ 撤销
                    </button>
                  )}
                  {idx > 0 && (
                    <button
                      className="redo-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        onRedo(cp);
                      }}
                      title="重做此修改"
                    >
                      ↪️ 重做
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default UndoRedoPanel;
