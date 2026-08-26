package perf

import (
	"sync"
)

// GoroutinePool 协程池
type GoroutinePool struct {
	workers   int
	taskQueue chan func()
	wg        sync.WaitGroup
	stop      chan struct{}
	running   bool
	lock      sync.Mutex
}

// NewGoroutinePool 创建协程池
func NewGoroutinePool(workers int, queueSize int) *GoroutinePool {
	return &GoroutinePool{
		workers:   workers,
		taskQueue: make(chan func(), queueSize),
		stop:      make(chan struct{}),
	}
}

// Start 启动协程池
func (p *GoroutinePool) Start() {
	p.lock.Lock()
	if p.running {
		p.lock.Unlock()
		return
	}
	p.running = true
	p.lock.Unlock()

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker 工作协程
func (p *GoroutinePool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			p.safeExec(task)
		case <-p.stop:
			// 处理队列中剩余的任务
			for {
				select {
				case task, ok := <-p.taskQueue:
					if !ok {
						return
					}
					p.safeExec(task)
				default:
					return
				}
			}
		}
	}
}

// safeExec 安全执行任务
func (p *GoroutinePool) safeExec(task func()) {
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 但不终止 worker
		}
	}()
	task()
}

// Submit 提交任务
func (p *GoroutinePool) Submit(task func()) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.running {
		return false
	}

	select {
	case p.taskQueue <- task:
		return true
	default:
		return false // 队列满
	}
}

// SubmitAndWait 提交任务并等待完成
func (p *GoroutinePool) SubmitAndWait(task func()) {
	done := make(chan struct{})
	p.Submit(func() {
		defer close(done)
		task()
	})
	<-done
}

// Stop 停止协程池
func (p *GoroutinePool) Stop() {
	p.lock.Lock()
	if !p.running {
		p.lock.Unlock()
		return
	}
	p.running = false
	p.lock.Unlock()

	close(p.stop)
	p.wg.Wait()
	close(p.taskQueue)
}

// Stats 池统计
type PoolStats struct {
	Workers    int
	QueueSize  int
	QueueLen   int
	IsRunning  bool
}

// Stats 获取池统计
func (p *GoroutinePool) Stats() PoolStats {
	p.lock.Lock()
	defer p.lock.Unlock()
	return PoolStats{
		Workers:   p.workers,
		QueueSize: cap(p.taskQueue),
		QueueLen:  len(p.taskQueue),
		IsRunning: p.running,
	}
}
