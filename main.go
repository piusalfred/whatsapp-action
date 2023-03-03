package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/piusalfred/whatsapp"
	whttp "github.com/piusalfred/whatsapp/http"
	"golang.org/x/sync/errgroup"
)

type (
	Inputs struct {
		BaseURL           string
		AccessToken       string
		Version           string
		PhoneNumberID     string
		BusinessAccountID string
		Recipients        []string
		Message           string
		PreviewURL        bool
		IgnoreErrors      bool
	}

	Response struct {
		StatusCode int
		Receiver   string
		MessageID  string
	}
)

func flattenResponse(receiver string, response *whttp.Response) *Response {
	messageID := ""
	if response != nil && response.Message != nil && len(response.Message.Messages) > 0 {
		messageID = response.Message.Messages[0].ID
	}
	return &Response{
		StatusCode: response.StatusCode,
		Receiver:   receiver,
		MessageID:  messageID,
	}
}

func Run(ctx context.Context, inputs *Inputs, responses chan<- *Response) error {
	recipients := inputs.Recipients
	nOfRecipients := len(recipients)
	if nOfRecipients == 0 {
		return fmt.Errorf("no recipients specified")
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	}

	httpClient := &http.Client{
		Transport: transport,
	}
	options := []whatsapp.ClientOption{
		whatsapp.WithHTTPClient(httpClient),
		whatsapp.WithBaseURL(inputs.BaseURL),
		whatsapp.WithAccessToken(inputs.AccessToken),
		whatsapp.WithVersion(inputs.Version),
		whatsapp.WithPhoneNumberID(inputs.PhoneNumberID),
		whatsapp.WithWhatsappBusinessAccountID(inputs.BusinessAccountID),
	}
	client := whatsapp.NewClient(options...)

	message := &whatsapp.TextMessage{
		Message:    inputs.Message,
		PreviewURL: inputs.PreviewURL,
	}

	errChan := make(chan error, len(inputs.Recipients))

	go func() {
		errChan <- run(ctx, client, inputs.Recipients, message, responses)
	}()

	allErrors := make([]error, 0, len(inputs.Recipients))

	// Wait for all goroutines to finish or for a signal to be received
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-errChan:
		for err := range errChan {
			allErrors = append(allErrors, err)
		}
		return errors.Join(allErrors...)
	}
}

func run(ctx context.Context, client *whatsapp.Client, recipients []string,
	message *whatsapp.TextMessage, responses chan<- *Response,
) error {
	errg, gctx := errgroup.WithContext(ctx)
	for _, recipient := range recipients {
		recipient := recipient
		message := message

		sendf := func(recipient string, message *whatsapp.TextMessage) func() error {
			return func() error {
				resp, err := client.SendTextMessage(gctx, recipient, message)
				if err != nil {
					return err
				}
				responses <- flattenResponse(recipient, resp)
				return nil
			}
		}
		errg.Go(sendf(recipient, message))
	}

	err := errg.Wait()
	defer close(responses)
	if err != nil {
		return err
	}

	return nil
}

////type readfd int
//
//func (r readfd) Read(buf []byte) (int, error) {
//	n, err := syscall.Read(int(r), buf)
//	if err != nil {
//		return -1, fmt.Errorf("read error: %w", err)
//	}
//
//	return n, nil
//}

type writefd int

func (w writefd) Write(buf []byte) (int, error) {
	n, err := syscall.Write(int(w), buf)
	if err != nil {
		return -1, fmt.Errorf("write error %w", err)
	}

	return n, nil
}

const (
	//stdin  = readfd(0)
	stdout = writefd(1)
	stderr = writefd(2)
)

func main() {
	inputs := &Inputs{
		BaseURL:           os.Getenv("INPUT_BASE_URL"),
		AccessToken:       os.Getenv("INPUT_ACCESS_TOKEN"),
		Version:           os.Getenv("INPUT_VERSION"),
		PhoneNumberID:     os.Getenv("INPUT_PHONE_NUMBER_ID"),
		BusinessAccountID: os.Getenv("INPUT_BUSINESS_ACCOUNT_ID"),
		Recipients:        strings.Split(os.Getenv("INPUT_RECIPIENTS"), ","),
		Message:           os.Getenv("INPUT_MESSAGE"),
		PreviewURL:        os.Getenv("INPUT_PREVIEW_URL") == "1",
	}

	ctx := context.Background()
	nctx, cancel := context.WithCancel(ctx)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	responseChan := make(chan *Response, len(inputs.Recipients))

	go func() {
		errChan <- Run(nctx, inputs, responseChan)
	}()

	select {
	case <-nctx.Done():
	case sig := <-signalChan:
		_, _ = fmt.Fprintf(stderr, "Received signal: %s\n", sig)
		cancel()
	}

	close(responseChan)

	for resp := range responseChan {
		_, _ = stdout.Write([]byte(fmt.Sprintf("response: %+v", resp)))
	}

	if err := <-errChan; err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
