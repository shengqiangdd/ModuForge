// ConfirmDialog.tsx - Confirmation dialog for sensitive operations
import React from 'react';

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  danger?: boolean;
  details?: string[];
}

export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  title,
  message,
  confirmText = '确认',
  cancelText = '取消',
  onConfirm,
  onCancel,
  danger = false,
  details,
}) => {
  if (!isOpen) return null;

  return (
    <div className="confirm-dialog-overlay">
      <div className={`confirm-dialog ${danger ? 'danger' : ''}`}>
        <div className="dialog-header">
          <span className="dialog-icon">{danger ? '⚠️' : '❓'}</span>
          <h3>{title}</h3>
        </div>
        <div className="dialog-body">
          <p>{message}</p>
          {details && details.length > 0 && (
            <ul className="dialog-details">
              {details.map((detail, idx) => (
                <li key={idx}>{detail}</li>
              ))}
            </ul>
          )}
        </div>
        <div className="dialog-actions">
          <button className="cancel-btn" onClick={onCancel}>
            {cancelText}
          </button>
          <button
            className={`confirm-btn ${danger ? 'danger' : ''}`}
            onClick={onConfirm}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
};

// Hook for managing confirmation state
export const useConfirm = () => {
  const [confirmState, setConfirmState] = React.useState<{
    isOpen: boolean;
    title: string;
    message: string;
    details?: string[];
    danger?: boolean;
    onConfirm?: () => void;
  }>({
    isOpen: false,
    title: '',
    message: '',
  });

  const confirm = React.useCallback(
    (options: {
      title: string;
      message: string;
      details?: string[];
      danger?: boolean;
    }) => {
      return new Promise<boolean>((resolve) => {
        setConfirmState({
          isOpen: true,
          ...options,
          onConfirm: () => {
            setConfirmState((prev) => ({ ...prev, isOpen: false }));
            resolve(true);
          },
        });
      });
    },
    []
  );

  const cancel = React.useCallback(() => {
    setConfirmState((prev) => ({ ...prev, isOpen: false }));
  }, []);

  return {
    ...confirmState,
    confirm,
    cancel,
  };
};

export default ConfirmDialog;
