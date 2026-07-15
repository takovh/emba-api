package sse

import (
	"encoding/json"
	"sync"
	"time"
)

type SSEMessage struct {
	ID      int
	Payload string
}

type eventPayload struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type SSEManager struct {
	mu       sync.RWMutex
	clients  map[string]map[string]chan<- SSEMessage
	history  map[string][]SSEMessage
	counters map[string]int
}

const bufferMax = 200

var Manager = NewSSEManager()

func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients:  make(map[string]map[string]chan<- SSEMessage),
		history:  make(map[string][]SSEMessage),
		counters: make(map[string]int),
	}
}

func (m *SSEManager) Register(taskID string, ch chan<- SSEMessage) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[taskID] == nil {
		m.clients[taskID] = make(map[string]chan<- SSEMessage)
	}
	clientID := randomID(8)
	m.clients[taskID][clientID] = ch
	return clientID
}

func (m *SSEManager) Unregister(taskID, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clients := m.clients[taskID]
	if clients != nil {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(m.clients, taskID)
		}
	}
}

func (m *SSEManager) Notify(taskID, msgType, message string) {
	payload := eventPayload{
		Type:      msgType,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	dataStr := string(data)

	m.mu.Lock()
	m.counters[taskID]++
	eventID := m.counters[taskID]

	buf := m.history[taskID]
	buf = append(buf, SSEMessage{ID: eventID, Payload: dataStr})
	if len(buf) > bufferMax {
		buf = buf[len(buf)-bufferMax:]
	}
	m.history[taskID] = buf

	clients := make(map[string]chan<- SSEMessage)
	for k, v := range m.clients[taskID] {
		clients[k] = v
	}
	m.mu.Unlock()

	for _, ch := range clients {
		select {
		case ch <- SSEMessage{ID: eventID, Payload: dataStr}:
		default:
		}
	}
}

func (m *SSEManager) GetAllHistory(taskID string) []SSEMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	buf := m.history[taskID]
	result := make([]SSEMessage, len(buf))
	copy(result, buf)
	return result
}

func (m *SSEManager) GetHistoryAfter(taskID string, afterID int) []SSEMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	buf := m.history[taskID]
	var result []SSEMessage
	for _, msg := range buf {
		if msg.ID > afterID {
			result = append(result, msg)
		}
	}
	return result
}

func (m *SSEManager) ClearHistory(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.history, taskID)
	delete(m.counters, taskID)
}

func (m *SSEManager) RemoveTaskAndClients(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, taskID)
	delete(m.history, taskID)
	delete(m.counters, taskID)
}
