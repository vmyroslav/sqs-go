package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/vmyroslav/sqs-go/consumer/observability"
)

type pollerConfig struct {
	MaxNumberOfMessages  int32
	WaitTimeSeconds      int32
	VisibilityTimeout    int32
	WorkerPoolSize       int32
	ErrorNumberThreshold int32
}

type sqsPoller struct {
	sqsClient sqsConnector
	tracer    observability.SQSTracer
	metrics   observability.SQSMetrics
	logger    *slog.Logger
	cfg       pollerConfig
}

func newSqsPoller(
	cfg pollerConfig,
	sqsClient sqsConnector,
	logger *slog.Logger,
	tracer observability.SQSTracer,
	metrics observability.SQSMetrics,
) *sqsPoller {
	return &sqsPoller{
		sqsClient: sqsClient,
		logger:    logger,
		cfg:       cfg,
		tracer:    tracer,
		metrics:   metrics,
	}
}

func (p *sqsPoller) Poll(parentCtx context.Context, queueURL string, ch chan<- sqstypes.Message) error {
	var (
		poolSize    = int(p.cfg.WorkerPoolSize)
		errCh       = make(chan error, poolSize)
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(parentCtx)
	)

	// ensures everything is cleaned on exit
	defer func() {
		close(ch)
		close(errCh)
		cancel()
	}()

	if poolSize == 0 {
		return errors.New("worker pool size should be greater than 0")
	}

	for i := 0; i < poolSize; i++ {
		wg.Add(1)

		go p.runWorker(ctx, &wg, errCh, cancel, queueURL, ch, i)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err // return the first error that occurred
	default:
		return nil
	}
}

// runWorker contains the main loop for a single poller goroutine
func (p *sqsPoller) runWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	errCh chan<- error,
	cancel context.CancelFunc,
	queueURL string,
	ch chan<- sqstypes.Message,
	workerIndex int,
) {
	defer wg.Done()

	var retryCount int32

	workerID := fmt.Sprintf("poller-%d", workerIndex)

	for {
		if ctx.Err() != nil {
			p.logger.DebugContext(ctx, "poller worker stopping due to context cancellation")

			return
		}

		start := time.Now()

		msgResult, err := p.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{
				sqstypes.MessageSystemAttributeNameAll,
			},
			MessageAttributeNames: []string{
				string(sqstypes.QueueAttributeNameAll),
			},
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: p.cfg.MaxNumberOfMessages,
			VisibilityTimeout:   p.cfg.VisibilityTimeout,
			WaitTimeSeconds:     p.cfg.WaitTimeSeconds,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				p.logger.DebugContext(ctx, "polling call canceled due to context cancellation")

				return
			}

			duration := time.Since(start)
			p.metrics.RecordDuration(ctx, observability.MetricPollingDuration, duration,
				observability.WithQueueURLMetric(queueURL),
			)
			p.metrics.Counter(ctx, observability.MetricPollingRequests, 1,
				observability.WithQueueURLMetric(queueURL),
				observability.WithStatus("failure"),
			)

			p.logger.ErrorContext(ctx, "failed to poll messages from SQS",
				"error", err,
				"queueURL", queueURL,
			)

			retryCount++

			// check if error threshold is reached (per-worker threshold)
			if p.cfg.ErrorNumberThreshold > 0 && retryCount >= p.cfg.ErrorNumberThreshold {
				errCh <- fmt.Errorf("worker poller error threshold reached (%d consecutive failures): %w", p.cfg.ErrorNumberThreshold, err)

				cancel()

				return
			}

			time.Sleep(p.backoff(retryCount))

			continue
		}

		// Record successful polling metrics
		duration := time.Since(start)
		p.metrics.RecordDuration(ctx, observability.MetricPollingDuration, duration,
			observability.WithQueueURLMetric(queueURL),
		)
		p.metrics.Counter(ctx, observability.MetricPollingRequests, 1,
			observability.WithQueueURLMetric(queueURL),
			observability.WithStatus("success"),
		)

		// reset retry count on success
		retryCount = 0

		messageCount := int64(len(msgResult.Messages))
		p.metrics.Gauge(ctx, observability.MetricMessagesReceived, messageCount,
			observability.WithQueueURLMetric(queueURL),
		)

		for _, msg := range msgResult.Messages {
			p.metrics.Counter(ctx, observability.MetricMessages, 1,
				observability.WithQueueURLMetric(queueURL),
				observability.WithWorkerID(workerID),
				observability.WithStatus("received"),
			)

			ch <- msg
		}
	}
}

// backoff implements a simple exponential backoff function.
func (p *sqsPoller) backoff(retryCount int32) time.Duration {
	baseDelay := 100 * time.Millisecond
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(retryCount)))

	maxDelay := 2000 * time.Millisecond
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
