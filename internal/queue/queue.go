package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// ProcessFileArgs holds arguments for the asynchronous file processing task.
type ProcessFileArgs struct {
	FileID       uuid.UUID `json:"file_id"`
	GenerateWebP bool      `json:"generate_webp"`
}

// Kind identifies the job kind for River dispatcher.
func (ProcessFileArgs) Kind() string { return "process_file" }

// FileProcessor defines the interface for executing file post-processing tasks.
type FileProcessor interface {
	ProcessAsync(ctx context.Context, fileID uuid.UUID, generateWebP bool) error
}

// ProcessFileWorker handles the background processing of uploaded files.
type ProcessFileWorker struct {
	river.WorkerDefaults[ProcessFileArgs]
	processor FileProcessor
}

// Work executes the file processing job.
func (w *ProcessFileWorker) Work(ctx context.Context, job *river.Job[ProcessFileArgs]) error {
	slog.Info("River: processing background file job",
		"file_id", job.Args.FileID,
		"generate_webp", job.Args.GenerateWebP,
		"attempt", job.Attempt,
	)

	if w.processor == nil {
		return nil
	}

	return w.processor.ProcessAsync(ctx, job.Args.FileID, job.Args.GenerateWebP)
}

// Queue manages background job execution and enqueueing.
type Queue interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	EnqueueProcessFile(ctx context.Context, fileID uuid.UUID, generateWebP bool) error
}

type riverQueue struct {
	client *river.Client[pgx.Tx]
}

// New initializes River background queue, runs migrations, and registers workers.
func New(ctx context.Context, pool *pgxpool.Pool, processor FileProcessor) (Queue, error) {
	if pool == nil {
		return &noopQueue{}, nil
	}

	// 1. Run River schema migration (direction: Up)
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize river schema migrator: %w", err)
	}

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to run river schema migrations: %w", err)
	}

	// 2. Register workers
	workers := river.NewWorkers()
	river.AddWorker(workers, &ProcessFileWorker{processor: processor})

	// 3. Initialize River Client
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize river client: %w", err)
	}

	slog.Info("River background job queue initialized successfully")
	return &riverQueue{client: client}, nil
}

func (q *riverQueue) Start(ctx context.Context) error {
	return q.client.Start(ctx)
}

func (q *riverQueue) Stop(ctx context.Context) error {
	return q.client.Stop(ctx)
}

func (q *riverQueue) EnqueueProcessFile(ctx context.Context, fileID uuid.UUID, generateWebP bool) error {
	_, err := q.client.Insert(ctx, ProcessFileArgs{
		FileID:       fileID,
		GenerateWebP: generateWebP,
	}, nil)
	return err
}

// noopQueue provides a safe fallback when database is not available.
type noopQueue struct{}

func (n *noopQueue) Start(_ context.Context) error { return nil }
func (n *noopQueue) Stop(_ context.Context) error  { return nil }
func (n *noopQueue) EnqueueProcessFile(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}
