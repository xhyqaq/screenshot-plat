package app

import (
	"net"
	"sync"
)

// 服务器运行期共享状态
type state struct {
	clients           map[net.Conn]string
	clientsMutex      sync.Mutex
	responseCollector chan string
	// 最近一次“已识别”的结果，用于 capture 模式下保留上次识别内容
	lastAnalyses []ImageEntry
	lastMu       sync.RWMutex
}

// getLastAnalyses 线程安全读取最近一次识别结果（浅拷贝）
func (a *App) getLastAnalyses() []ImageEntry {
	a.lastMu.RLock()
	defer a.lastMu.RUnlock()
	out := make([]ImageEntry, len(a.lastAnalyses))
	copy(out, a.lastAnalyses)
	return out
}

// setLastAnalyses 线程安全设置最近一次识别结果（深拷贝 ModelAnswers）
func (a *App) setLastAnalyses(in []ImageEntry) {
	a.lastMu.Lock()
	defer a.lastMu.Unlock()
	a.lastAnalyses = make([]ImageEntry, len(in))
	for i := range in {
		ent := ImageEntry{Base64: in[i].Base64}
		if len(in[i].ModelAnswers) > 0 {
			ent.ModelAnswers = append([]ModelAnswer(nil), in[i].ModelAnswers...)
		}
		a.lastAnalyses[i] = ent
	}
}
