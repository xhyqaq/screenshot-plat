package app

import _ "embed"

// 默认内置模板，保证仅携带二进制也能运行
//
//go:embed templates/result_default.html
var defaultTemplate []byte
