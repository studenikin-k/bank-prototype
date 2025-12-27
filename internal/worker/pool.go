package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"bank-prototype/internal/utils"
)

// Job представляет задачу для выполнения
type Job struct {
	ID      string
	Task    func() error
	RetryOn func(error) bool // Функция для определения, нужна ли повторная попытка
	OnDone  func(error)      // Callback после завершения
}

// WorkerPool управляет пулом воркеров
type WorkerPool struct {
	workers    int
	jobQueue   chan Job
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	stats      PoolStats
	maxRetries int
}

// PoolStats содержит статистику работы пула
type PoolStats struct {
	TotalJobs     int64
	CompletedJobs int64
	FailedJobs    int64
	ActiveWorkers int
	QueuedJobs    int
}

func NewWorkerPool(workers int, queueSize int, maxRetries int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:    workers,
		jobQueue:   make(chan Job, queueSize),
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: maxRetries,
		stats: PoolStats{
			ActiveWorkers: workers,
		},
	}

	utils.LogSuccess("WorkerPool", "Создан пул воркеров")
	utils.LogInfo("WorkerPool", "Количество воркеров: %d", workers)
	utils.LogInfo("WorkerPool", "Размер очереди: %d", queueSize)
	utils.LogInfo("WorkerPool", "Максимум повторов: %d", maxRetries)

	return pool
}

// Start запускает воркеры
func (p *WorkerPool) Start() {
	utils.LogInfo("WorkerPool", "Запуск воркеров...")

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	utils.LogSuccess("WorkerPool", "Все воркеры запущены")
}

// worker - функция воркера, обрабатывающая задачи
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	utils.LogInfo("WorkerPool", "Воркер #%d запущен", id)

	for {
		select {
		case <-p.ctx.Done():
			utils.LogInfo("WorkerPool", "Воркер #%d завершает работу", id)
			return

		case job, ok := <-p.jobQueue:
			if !ok {
				utils.LogInfo("WorkerPool", "Воркер #%d: очередь закрыта", id)
				return
			}

			p.updateStats(0, -1) // Уменьшаем счетчик очереди
			p.executeJob(id, job)
		}
	}
}

// executeJob выполняет задачу с повторными попытками при необходимости
func (p *WorkerPool) executeJob(workerID int, job Job) {
	startTime := time.Now()
	var err error

	// Попытки выполнения с повторами
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			utils.LogWarning("WorkerPool", "Воркер #%d: повторная попытка #%d для задачи %s", workerID, attempt, job.ID)
			time.Sleep(time.Millisecond * time.Duration(100*attempt)) // Экспоненциальная задержка
		}

		err = job.Task()

		if err == nil {
			// Успешное выполнение
			p.updateStats(1, 0)
			duration := time.Since(startTime)
			utils.LogSuccess("WorkerPool", "Воркер #%d: задача %s выполнена за %v", workerID, job.ID, duration)

			if job.OnDone != nil {
				job.OnDone(nil)
			}
			return
		}

		// Проверяем, нужна ли повторная попытка
		if job.RetryOn != nil && !job.RetryOn(err) {
			break // Ошибка не требует повтора
		}
	}

	// Если все попытки исчерпаны или ошибка не требует повтора
	p.updateStats(0, 0)
	p.mu.Lock()
	p.stats.FailedJobs++
	p.mu.Unlock()

	duration := time.Since(startTime)
	utils.LogError("WorkerPool", fmt.Sprintf("Воркер #%d: задача %s провалилась после %v", workerID, job.ID, duration), err)

	if job.OnDone != nil {
		job.OnDone(err)
	}
}

// Submit добавляет задачу в очередь
func (p *WorkerPool) Submit(job Job) error {
	select {
	case <-p.ctx.Done():
		return context.Canceled

	case p.jobQueue <- job:
		p.updateStats(0, 1) // Увеличиваем счетчик очереди
		utils.LogDebug("WorkerPool", "Задача %s добавлена в очередь (в очереди: %d)", job.ID, p.GetStats().QueuedJobs)
		return nil

	default:
		utils.LogWarning("WorkerPool", "Очередь переполнена, задача %s отклонена", job.ID)
		return ErrQueueFull
	}
}

// SubmitBlocking добавляет задачу в очередь с блокировкой
func (p *WorkerPool) SubmitBlocking(job Job) error {
	select {
	case <-p.ctx.Done():
		return context.Canceled

	case p.jobQueue <- job:
		p.updateStats(0, 1)
		utils.LogDebug("WorkerPool", "Задача %s добавлена в очередь (блокирующий режим)", job.ID)
		return nil
	}
}

// Shutdown останавливает пул воркеров
func (p *WorkerPool) Shutdown(timeout time.Duration) error {
	utils.LogInfo("WorkerPool", "Начинается остановка пула воркеров...")

	// Закрываем очередь для новых задач
	close(p.jobQueue)

	// Канал для ожидания завершения всех воркеров
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Ждем завершения с таймаутом
	select {
	case <-done:
		utils.LogSuccess("WorkerPool", "Все воркеры завершили работу")
		return nil

	case <-time.After(timeout):
		p.cancel() // Принудительно завершаем воркеры
		log.Printf("[WARNING] [WorkerPool] Превышен таймаут остановки, принудительное завершение")
		return ErrShutdownTimeout
	}
}

// GetStats возвращает текущую статистику пула
func (p *WorkerPool) GetStats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := p.stats
	stats.QueuedJobs = len(p.jobQueue)
	return stats
}

// updateStats обновляет статистику пула
func (p *WorkerPool) updateStats(completed int64, queued int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalJobs++
	p.stats.CompletedJobs += completed

	if queued > 0 {
		p.stats.QueuedJobs += queued
	} else if queued < 0 {
		p.stats.QueuedJobs += queued
	}
}

var (
	ErrQueueFull       = context.DeadlineExceeded
	ErrShutdownTimeout = context.DeadlineExceeded
)

// GetCurrentTimeMs возвращает текущее время в миллисекундах (Unix timestamp)
func GetCurrentTimeMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
