const MAX_STEPS = 50;

export interface HistoryAction {
  type: string;
  data: any;
  timestamp: number;
}

class HistoryStore {
  private stack: HistoryAction[] = [];
  private index = -1;

  push(action: Omit<HistoryAction, 'timestamp'>) {
    this.stack = this.stack.slice(0, this.index + 1);
    this.stack.push({ ...action, timestamp: Date.now() });
    if (this.stack.length > MAX_STEPS) {
      this.stack.shift();
    }
    this.index = this.stack.length - 1;
  }

  undo(): HistoryAction | null {
    if (this.index < 0) return null;
    const action = this.stack[this.index];
    this.index--;
    return action;
  }

  redo(): HistoryAction | null {
    if (this.index >= this.stack.length - 1) return null;
    this.index++;
    return this.stack[this.index];
  }

  getHistory(): HistoryAction[] {
    return this.stack;
  }

  getUndoCount(): number {
    return this.index + 1;
  }

  getRedoCount(): number {
    return this.stack.length - this.index - 1;
  }

  canUndo(): boolean {
    return this.index >= 0;
  }

  canRedo(): boolean {
    return this.index < this.stack.length - 1;
  }

  clear() {
    this.stack = [];
    this.index = -1;
  }
}

export const historyStore = new HistoryStore();
