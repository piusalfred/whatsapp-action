package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	whatsapp "github.com/piusalfred/whatsapp"
	whttp "github.com/piusalfred/whatsapp/http"
	errgroup "golang.org/x/sync/errgroup"
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
		StatusCode  int
		PhoneNumber string
	}
)

func Run(ctx context.Context, inputs *Inputs) error {
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

	errChan := make(chan error, 1)
	responseChan := make(chan *whttp.Response, nOfRecipients)
	// before exiting, print all responses
	defer func() {
		for resp := range responseChan {
			stdout.Write([]byte(fmt.Sprintf("response: %+v", resp)))
		}
	}()
	go func() {
		errChan <- run(ctx, client, inputs.Recipients, message, responseChan)
	}()

	// Wait for all goroutines to finish or for a signal to be received
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func run(ctx context.Context, client *whatsapp.Client, recipients []string,
	message *whatsapp.TextMessage, responses chan<- *whttp.Response,
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
				responses <- resp
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

type readfd int

func (r readfd) Read(buf []byte) (int, error) {
	n, err := syscall.Read(int(r), buf)
	if err != nil {
		return -1, fmt.Errorf("pesakit base io read error: %w", err)
	}

	return n, nil
}

type writefd int

func (w writefd) Write(buf []byte) (int, error) {
	n, err := syscall.Write(int(w), buf)
	if err != nil {
		return -1, fmt.Errorf("bedrock/io write error %w", err)
	}

	return n, nil
}

const (
	stdin  = readfd(0)
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
	defer cancel()

	if err := Run(nctx, inputs); err != nil {
		fmt.Fprintf(stderr, "error: %s", err.Error())
		os.Exit(1)
	}
}
