package app

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"
)

// startHTTPServer 注册路由并启动 HTTP 服务
func (a *App) startHTTPServer() {
	http.HandleFunc("/one", a.handleOne)
	if err := http.ListenAndServe(":8848", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}

func (a *App) handleOne(w http.ResponseWriter, r *http.Request) {
	// 诊断：打印配置摘要，确认运行期可见 key/baseURL/模板路径
	fmt.Fprintf(os.Stderr, "handleOne: models=%v baseURL=%s keylen=%d tpl=%s\n", a.cfg.Models, a.cfg.SiliconflowBaseURL, len(a.cfg.SiliconflowAPIKey), a.cfg.TemplatePath)

	// 支持两种模式：mode=capture 仅截屏；mode=analyze 截屏并识别（默认）
	mode := r.URL.Query().Get("mode")
	analyze := (mode == "" || mode == "analyze")

	a.clientsMutex.Lock()
	numClients := len(a.clients)
	a.clientsMutex.Unlock()
	if numClients == 0 {
		http.Error(w, "No connected clients", http.StatusBadRequest)
		return
	}

	a.sendCaptureCommandToClients()

	// 等待所有客户端的响应
	var allResponses []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func() {
			defer wg.Done()
			select {
			case responseStr := <-a.responseCollector:
				mu.Lock()
				allResponses = append(allResponses, responseStr)
				mu.Unlock()
			case <-time.After(10 * time.Second):
				fmt.Println("Timeout waiting for client response")
			}
		}()
	}
	wg.Wait()

	// 根据模式决定是否进行识别
	var analyses []ImageEntry
	if analyze {
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		analyses = a.analyzeImages(ctx, allResponses)
		// 识别后缓存为“最近一次已识别”
		a.setLastAnalyses(analyses)
	} else {
		// 仅截屏模式：合并“新截图的 Base64”与“上一次识别结果的 ModelAnswers”，保留既有识别
		last := a.getLastAnalyses()
		analyses = make([]ImageEntry, len(allResponses))
		for i := range allResponses {
			analyses[i] = ImageEntry{Base64: allResponses[i]}
			if i < len(last) && len(last[i].ModelAnswers) > 0 {
				analyses[i].ModelAnswers = append([]ModelAnswer(nil), last[i].ModelAnswers...)
			}
		}
	}

	// HTML 渲染：模板外置
	type PageData struct{ Items []ImageEntry }
	data := PageData{Items: analyses}
	tplBytes, err := os.ReadFile(a.cfg.TemplatePath)
	if err != nil || len(tplBytes) == 0 {
		// 不存在外部模板时回退到内置模板，确保单文件二进制可运行
		tplBytes = defaultTemplate
	}
	tmpl, err := template.New("result").Parse(string(tplBytes))
	if err != nil {
		http.Error(w, "Internal Server Error: unable to parse template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error: unable to execute template", http.StatusInternalServerError)
		return
	}
}
