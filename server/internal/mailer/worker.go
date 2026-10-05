package mailer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var (
	ErrQueueFull   = errors.New("mailer: email worker queue is full")
	ErrWorkerClose = errors.New("mailer: email worker pool is shutting down or stopped")
)

type jobType int

const (
	jobTypeSingle jobType = iota
	jobTypeBatch
)

type emailJob struct {
	kind       jobType
	single     *Email
	batch      []*Email
	retries    int
	maxRetries int
}

// WorkerPoolConfig defines settings for the concurrent email worker group.
type WorkerPoolConfig struct {
	WorkerCount int
	QueueSize   int
	MaxRetries  int
	RetryDelay  time.Duration
}

// WorkerPool handles asynchronous background dispatch and batch processing of emails.
type WorkerPool struct {
	cfg       WorkerPoolConfig
	sender    Sender
	logger    *slog.Logger
	queue     chan emailJob
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	mu        sync.Mutex
}

// NewWorkerPool creates a new WorkerPool.
func NewWorkerPool(cfg WorkerPoolConfig, sender Sender, logger *slog.Logger) *WorkerPool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 5
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}

	return &WorkerPool{
		cfg:    cfg,
		sender: sender,
		logger: logger,
		queue:  make(chan emailJob, cfg.QueueSize),
	}
}

// Start launches the worker group goroutines.
func (wp *WorkerPool) Start(parentCtx context.Context) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.isRunning {
		return
	}

	wp.ctx, wp.cancel = context.WithCancel(parentCtx)
	wp.isRunning = true

	if wp.logger != nil {
		wp.logger.Info("Starting email worker group",
			slog.Int("workers", wp.cfg.WorkerCount),
			slog.Int("queue_size", wp.cfg.QueueSize),
		)
	}

	for i := 1; i <= wp.cfg.WorkerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Enqueue adds an email to the worker queue for asynchronous dispatch.
func (wp *WorkerPool) Enqueue(email *Email) error {
	wp.mu.Lock()
	if !wp.isRunning {
		wp.mu.Unlock()
		return ErrWorkerClose
	}
	wp.mu.Unlock()

	job := emailJob{
		kind:       jobTypeSingle,
		single:     email,
		maxRetries: wp.cfg.MaxRetries,
	}

	select {
	case wp.queue <- job:
		return nil
	default:
		if wp.logger != nil {
			wp.logger.Warn("Email queue is full, dropping or backpressuring", slog.Any("to", email.To))
		}
		return ErrQueueFull
	}
}

// EnqueueBatch adds a batch of emails to the worker queue for asynchronous batch dispatch.
func (wp *WorkerPool) EnqueueBatch(emails []*Email) error {
	if len(emails) == 0 {
		return nil
	}

	wp.mu.Lock()
	if !wp.isRunning {
		wp.mu.Unlock()
		return ErrWorkerClose
	}
	wp.mu.Unlock()

	job := emailJob{
		kind:       jobTypeBatch,
		batch:      emails,
		maxRetries: wp.cfg.MaxRetries,
	}

	select {
	case wp.queue <- job:
		return nil
	default:
		if wp.logger != nil {
			wp.logger.Warn("Email queue is full during batch enqueue", slog.Int("batch_size", len(emails)))
		}
		return ErrQueueFull
	}
}

// Stop gracefully stops the worker pool, processing all queued items before exiting.
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	if !wp.isRunning {
		wp.mu.Unlock()
		return
	}
	wp.isRunning = false
	wp.mu.Unlock()

	if wp.logger != nil {
		wp.logger.Info("Stopping email worker group gracefully, draining queue...")
	}

	close(wp.queue)
	wp.wg.Wait()

	if wp.cancel != nil {
		wp.cancel()
	}

	if wp.logger != nil {
		wp.logger.Info("Email worker group stopped successfully")
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	defer func() {
		if rvr := recover(); rvr != nil {
			if wp.logger != nil {
				wp.logger.Error("Email worker panicked, restarting worker",
					slog.Int("worker_id", id),
					slog.Any("panic", rvr),
				)
			}
			wp.mu.Lock()
			running := wp.isRunning
			wp.mu.Unlock()
			if running {
				wp.wg.Add(1)
				go wp.worker(id)
			}
		}
	}()

	for job := range wp.queue {
		wp.processJob(id, job)
	}
}

func (wp *WorkerPool) processJob(workerID int, job emailJob) {
	sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	if job.kind == jobTypeSingle {
		err = wp.sender.Send(sendCtx, job.single)
	} else {
		err = wp.sender.SendBatch(sendCtx, job.batch)
	}

	if err != nil {
		if job.retries < job.maxRetries {
			job.retries++
			backoff := wp.cfg.RetryDelay * time.Duration(1<<job.retries)
			if wp.logger != nil {
				wp.logger.Warn("Failed to send email, scheduling retry",
					slog.Int("worker_id", workerID),
					slog.Int("retry_count", job.retries),
					slog.Duration("backoff", backoff),
					slog.String("error", err.Error()),
				)
			}
			time.Sleep(backoff)

			// Re-enqueue for retry
			wp.mu.Lock()
			if wp.isRunning {
				select {
				case wp.queue <- job:
				default:
					if wp.logger != nil {
						wp.logger.Error("Queue full during retry re-enqueue, dropped job", slog.String("error", err.Error()))
					}
				}
			}
			wp.mu.Unlock()
			return
		}

		if wp.logger != nil {
			wp.logger.Error("Email dispatch permanently failed after retries",
				slog.Int("worker_id", workerID),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	if wp.logger != nil {
		if job.kind == jobTypeSingle {
			wp.logger.Debug("Email sent successfully by worker",
				slog.Int("worker_id", workerID),
				slog.Any("to", job.single.To),
				slog.String("subject", job.single.Subject),
			)
		} else {
			wp.logger.Debug("Batch email sent successfully by worker",
				slog.Int("worker_id", workerID),
				slog.Int("count", len(job.batch)),
			)
		}
	}
}
