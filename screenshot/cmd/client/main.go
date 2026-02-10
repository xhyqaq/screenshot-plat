package main

import "screenshot/internal/app"

// 修改远程部署时的服务端地址即可
var address = "127.0.0.1:12345"

func main() { app.Run(address) }
