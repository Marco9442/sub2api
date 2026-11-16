package main

import (
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accountsim"
	"github.com/Wei-Shaw/sub2api/internal/handler"
)

// startAccountSim 启动账号管理 #2 的后台演进，并把接口挂到管理路由上。
// Wire 会在 ProvideHandlers 之前调用它，因此注册路由时 AccountSim 已经挂上。
func startAccountSim(db *sql.DB, admin *handler.AdminHandlers) handler.AccountSimStop {
	svc := accountsim.NewService(accountsim.NewStore(db), time.Now)
	svc.Start()
	if admin != nil {
		admin.AccountSim = accountsim.NewHandler(svc)
	}
	return svc.Stop
}
