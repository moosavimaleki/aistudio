package aistudio

import "context"

type Client struct {
	Settings  Settings
	HTTP      *HTTPClient
	ActiveTab *Tab
	State     TabState
}

func NewClient(settings Settings, httpClient *HTTPClient) *Client {
	return &Client{Settings: settings, HTTP: httpClient, State: TabNew}
}
func (c *Client) Initialize(ctx context.Context) error {
	if c.ActiveTab != nil {
		return NewError("CONFIG", "Client is already initialized")
	}
	tab, err := NewTab(c.Settings, c.HTTP, "")
	if err != nil {
		return err
	}
	if _, err = tab.Initialize(ctx); err != nil {
		c.State = tab.State
		return err
	}
	c.ActiveTab, c.State = tab, TabReady
	return nil
}
func (c *Client) Generate(ctx context.Context, input GenerateInput, onChunk func(any)) (GenerateResult, error) {
	if c.ActiveTab == nil || c.ActiveTab.State != TabReady {
		return GenerateResult{}, NewError("RPC", "Client is not ready")
	}
	result, err := c.ActiveTab.Generate(ctx, input, onChunk)
	if !InvalidatesTab(err) {
		return result, err
	}
	c.ActiveTab = nil
	c.State = TabInvalid
	if retryErr := c.Initialize(ctx); retryErr != nil {
		return GenerateResult{}, retryErr
	}
	return c.ActiveTab.Generate(ctx, input, onChunk)
}
func (c *Client) Close() {
	if c.ActiveTab != nil {
		c.ActiveTab.State = TabClosed
		c.ActiveTab = nil
	}
	c.State = TabClosed
}
