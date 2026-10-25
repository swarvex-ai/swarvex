package main

// Minimaler MCP-Client (Streamable HTTP, JSON-Antworten) für die Werkzeuge der Plattform.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

type mcpClient struct {
	basis string
	token string
	http  *http.Client
	id    atomic.Int64
}

type werkzeugFehler struct{ text string }

func (f werkzeugFehler) Error() string { return f.text }

func (m *mcpClient) aufruf(methode string, params any) (json.RawMessage, error) {
	koerper, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": m.id.Add(1), "method": methode, "params": params})
	req, _ := http.NewRequest(http.MethodPost, m.basis+"/mcp", bytes.NewReader(koerper))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json, text/event-stream")
	req.Header.Set("authorization", "Bearer "+m.token)
	r, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	roh, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP %s: HTTP %d: %s", methode, r.StatusCode, roh)
	}
	var antwort struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(roh, &antwort); err != nil {
		return nil, fmt.Errorf("MCP %s: %v", methode, err)
	}
	if antwort.Error != nil {
		return nil, fmt.Errorf("MCP %s: %s", methode, antwort.Error.Message)
	}
	return antwort.Result, nil
}

func (m *mcpClient) initialisieren() error {
	_, err := m.aufruf("initialize", map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "swarvex", "version": "0.1.0"},
	})
	return err
}

// werkzeug ruft ein MCP-Werkzeug auf und dekodiert dessen Text-Ergebnis (JSON) nach aus.
func (m *mcpClient) werkzeug(name string, argumente map[string]any, aus any) error {
	roh, err := m.aufruf("tools/call", map[string]any{"name": name, "arguments": argumente})
	if err != nil {
		return err
	}
	var ergebnis struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(roh, &ergebnis); err != nil {
		return err
	}
	text := ""
	if len(ergebnis.Content) > 0 {
		text = ergebnis.Content[0].Text
	}
	if ergebnis.IsError {
		return werkzeugFehler{text}
	}
	if aus == nil {
		return nil
	}
	if err := json.Unmarshal([]byte(text), aus); err != nil {
		return fmt.Errorf("%s: unerwartete Antwort: %s", name, text)
	}
	return nil
}
