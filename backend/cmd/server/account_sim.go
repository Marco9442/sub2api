package main

import (
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accountsim"
	"github.com/Wei-Shaw/sub2api/internal/handler"
)

// startAccountSim 启动账号管理 #2 的后台演进，并把接口挂到管理路由上。
// 返回的函数停止演进。调用点在 wire_gen.go，不经过 Wire。
func startAccountSim(db *sql.DB, admin *handler.AdminHandlers) func() {
	svc := accountsim.NewService(accountsim.NewStore(db), time.Now)
	svc.Start()
	if admin != nil {
		admin.AccountSim = accountsim.NewHandler(svc)
	}
	return svc.Stop
}
