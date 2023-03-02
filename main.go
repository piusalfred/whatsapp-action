package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type (
	Response struct {
		StatusCode int                 `json:"status_code,omitempty"`
		Headers    map[string][]string `json:"headers,omitempty"`
		Message    *ResponseMessage    `json:"message,omitempty"`
	}

	ResponseMessage struct {
		Product  string             `json:"messaging_product,omitempty"`
		Contacts []*ResponseContact `json:"contacts,omitempty"`
		Messages []*MessageID       `json:"messages,omitempty"`
	}


	ResponseError struct {
		Code int            `json:"code,omitempty"`
		Err  *Error `json:"error,omitempty"`
	}


	MessageID struct {
		ID string `json:"id,omitempty"`
	}

	ResponseContact struct {
		Input      string `json:"input"`
		WhatsappID string `json:"wa_id"`
	}

	Text struct {
		PreviewUrl bool   `json:"preview_url,omitempty"`
		Body       string `json:"body,omitempty"`
	}

	Message struct {
		Product       string `json:"messaging_product"`
		To            string `json:"to"`
		RecipientType string `json:"recipient_type"`
		Type          string `json:"type"`
		PreviewURL    bool   `json:"preview_url,omitempty"`
		Text          *Text  `json:"text,omitempty"`
	}
	Context struct {
		BaseURL           string
		AccessToken       string
		Version           string
		PhoneNumberID     string
		BusinessAccountID string
		Recipients        []string
		Message           string
		PreviewURL        bool
	}

	Client struct {
		mu                    *sync.Mutex
		HTTP                      *http.Client
		BaseURL                   string
		Version                   string
		AccessToken               string
		PhoneNumberID             string
		WhatsappBusinessAccountID string
	}

	ClientOption func(*Client)
)

func WithHTTPClient(http *http.Client) ClientOption {
	return func(client *Client) {
		client.HTTP = http
	}
}

func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		client.BaseURL = baseURL
	}
}

func WithVersion(version string) ClientOption {
	return func(client *Client) {
		client.Version = version
	}
}

func WithAccessToken(accessToken string) ClientOption {
	return func(client *Client) {
		client.AccessToken = accessToken
	}
}

func WithPhoneNumberID(phoneNumberID string) ClientOption {
	return func(client *Client) {
		client.PhoneNumberID = phoneNumberID
	}
}

func WithWhatsappBusinessAccountID(whatsappBusinessAccountID string) ClientOption {
	return func(client *Client) {
		client.WhatsappBusinessAccountID = whatsappBusinessAccountID
	}
}

func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		mu:                       &sync.Mutex{},
		HTTP:                      http.DefaultClient,
		BaseURL:                   BaseURL,
		Version:                   "v16.0",
		AccessToken:               "",
		PhoneNumberID:             "",
		WhatsappBusinessAccountID: "",
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) SetAccessToken(accessToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AccessToken = accessToken
}

func (c *Client) SetPhoneNumberID(phoneNumberID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PhoneNumberID = phoneNumberID
}

func (c *Client) SetWhatsappBusinessAccountID(whatsappBusinessAccountID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WhatsappBusinessAccountID = whatsappBusinessAccountID
}

type TextMessage struct {
	Message    string
	PreviewURL bool
}

// SendTextMessage sends a text message to a WhatsApp Business Account.
func (c *Client) SendTextMessage(ctx context.Context, recipient string, message *TextMessage) (*whttp.Response, error) {
	httpC := c.HTTP
	request := &SendTextRequest{
		BaseURL:       c.BaseURL,
		AccessToken:   c.AccessToken,
		PhoneNumberID: c.PhoneNumberID,
		ApiVersion:    c.Version,
		Recipient:     recipient,
		Message:       message.Message,
		PreviewURL:    message.PreviewURL,
	}
	resp, err := SendText(ctx, httpC, request)
	if err != nil {
		return nil, fmt.Errorf("failed to send text message: %w", err)
	}

	return resp, nil
}

func NewRequestWithContext(ctx context.Context, method, requestURL string, params *RequestParams,
	payload []byte,
) (*http.Request, error) {
	var (
		body io.Reader
		req  *http.Request
	)
	if params.Form != nil {
		form := url.Values{}
		for key, value := range params.Form {
			form.Add(key, value)
		}
		body = strings.NewReader(form.Encode())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if payload != nil {
		body = bytes.NewReader(payload)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Set the request headers
	if params.Headers != nil {
		for key, value := range params.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set the bearer token header
	if params.Bearer != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", params.Bearer))
	}

	// Add the query parameters to the request URL
	if params.Query != nil {
		query := req.URL.Query()
		for key, value := range params.Query {
			query.Add(key, value)
		}
		req.URL.RawQuery = query.Encode()
	}

	return req, nil
}

// createRequestURL creates a new request url by joining the base url, api version
// sender id and endpoints.
// It is called by the NewRequestWithContext function where these details are
// passed from the RequestParams.
func createRequestURL(baseURL, apiVersion, senderID string, endpoints ...string) (string, error) {
	elems := append([]string{apiVersion, senderID}, endpoints...)

	path, err := url.JoinPath(baseURL, elems...)
	if err != nil {
		return "", fmt.Errorf("failed to join url path: %w", err)
	}

	return path, nil
}

func (c *Client)SendText(ctx context.Context,recipient string, message *Text) (*whttp.Response, error) {
	httpC := c.HTTP
	request := &SendTextRequest{
		BaseURL:       c.BaseURL,
		AccessToken:   c.AccessToken,
		PhoneNumberID: c.PhoneNumberID,
		ApiVersion:    c.Version,
		Recipient:     recipient,
		Message:       message.Message,
		PreviewURL:    message.PreviewURL,
	}
	resp, err := SendText(ctx, httpC, request)
	if err != nil {
		return nil, fmt.Errorf("failed to send text message: %w", err)
	}

	return resp, nil
}

}

func main() {
	context := &Context{
		BaseURL:           os.Getenv("INPUT_BASE_URL"),
		AccessToken:       os.Getenv("INPUT_ACCESS_TOKEN"),
		Version:           os.Getenv("INPUT_VERSION"),
		PhoneNumberID:     os.Getenv("INPUT_PHONE_NUMBER_ID"),
		BusinessAccountID: os.Getenv("INPUT_BUSINESS_ACCOUNT_ID"),
		Recipients:        strings.Split(os.Getenv("INPUT_RECIPIENTS"), ","),
		Message:           os.Getenv("INPUT_MESSAGE"),
		PreviewURL:        os.Getenv("INPUT_PREVIEW_URL") == "true",
	}
	fmt.Println("Hello, World!")
}
