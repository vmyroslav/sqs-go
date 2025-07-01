package consumer

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type pollerConfig struct {
	MaxNumberOfMessages  int32
	WaitTimeSeconds      int32
	VisibilityTimeout    int32
	WorkerPoolSize       int32
	ErrorNumberThreshold int32
}

type sqsPoller struct { // nolint:govet
	cfg       pollerConfig
	sqsClient sqsConnector
	logger    *slog.Logger
}

func newSqsPoller(
	cfg pollerConfig,
	sqsClient sqsConnector,
	logger *slog.Logger,
) *sqsPoller {
	return &sqsPoller{
		sqsClient: sqsClient,
		logger:    logger,
		cfg:       cfg,
	}
}

func (p *sqsPoller) Poll(parentCtx context.Context, queueURL string, ch chan<- sqstypes.Message) error { //nolint: cyclop
	var (
		poolSize = int(p.cfg.WorkerPoolSize)
		errCh    = make(chan error, poolSize)
		wg       sync.WaitGroup

		ctx, cancel = context.WithCancel(parentCtx)
	)

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

		go func() { // nolint: wsl
			defer wg.Done()

			var retryCount int32

			for {
				select {
				case <-ctx.Done():
					p.logger.DebugContext(ctx, "poller stopped. Context is canceled.")
					return
				default:
				}

				msgResult, err := p.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
					AttributeNames: []sqstypes.QueueAttributeName{
						sqstypes.QueueAttributeNameAll,
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
						p.logger.DebugContext(ctx, "poller stopped. Context is canceled.")

						return
					}

					p.logger.ErrorContext(
						ctx,
						"failed to poll messages from SQS",
						"error",
						err,
						"queueURL",
						queueURL,
					)

					retryCount++
					// if the error threshold is enabled and the number of retries is greater than the threshold, stop the poller
					if p.cfg.ErrorNumberThreshold > 0 && retryCount >= p.cfg.ErrorNumberThreshold {
						errCh <- errors.New("error threshold reached, stopping consumer")

						cancel()

						return
					}

					time.Sleep(p.backoff(retryCount))

					continue
				}

				retryCount = 0

				for _, msg := range msgResult.Messages {
					// if the context is canceled we will produce the received messages
					// to the channel and return on the next iteration
					ch <- msg
				}
			}
		}()
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err // return the first error occurred
	default:
		return nil
	}
}

// backoff a simple exponential backoff function.
func (p *sqsPoller) backoff(retryCount int32) time.Duration {
	baseDelay := 100 * time.Millisecond

	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(retryCount)))

	maxDelay := 2000 * time.Millisecond
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
